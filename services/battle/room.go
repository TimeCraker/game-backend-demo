package battle

import (
	"time"

	pb "github.com/TimeCraker/game-backend-demo/services/proto"
	"google.golang.org/protobuf/proto"
)

// ===== 新增代码 START =====
// 修改内容：定义战斗房间输入事件结构
// 修改原因：将网关上行输入转为强类型结构，便于在固定帧中消费
// 影响范围：battle.Room 输入处理与后续网关接入层
type InputEvent struct {
	UserID uint32
	Input  InputSnapshot
}

// BattleRoom 表示一个服务端权威战斗房间（当前脚手架支持 1v1）。
type BattleRoom struct {
	// RoomID：房间唯一标识，与匹配系统产出的 room_id 一致
	RoomID string
	// Player1/Player2：当前房间的两名对战玩家状态
	Player1 BattlePlayer
	Player2 BattlePlayer

	// InputCh：输入通道，由网关将客户端输入投递到该通道
	InputCh chan InputEvent
	// BroadcastCh：每帧下行快照通道，供网关消费并广播给前端
	BroadcastCh chan []byte
	// stopCh：用于优雅停止房间 tick 循环
	stopCh chan struct{}
}

// NewBattleRoom 创建一个可运行的战斗房间脚手架。
func NewBattleRoom(roomID string, p1ID, p2ID uint32) *BattleRoom {
	return &BattleRoom{
		RoomID: roomID,
		Player1: BattlePlayer{
			UserID:       p1ID,
			CurrentState: Idle,
			Position:     Vector2{X: -300, Z: 0},
			HP:           100,
			Energy:       0,
			FacingX:      1.0,
		},
		Player2: BattlePlayer{
			UserID:       p2ID,
			CurrentState: Idle,
			Position:     Vector2{X: 300, Z: 0},
			HP:           100,
			Energy:       0,
			FacingX:      -1.0,
		},
		InputCh:     make(chan InputEvent, 128),
		BroadcastCh: make(chan []byte, 128),
		stopCh:      make(chan struct{}),
	}
}

// Start 以 60Hz 固定帧推进战斗逻辑，并在每帧输出 state 快照。
func (r *BattleRoom) Start() {
	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()

	last := time.Now()
	for {
		select {
		case <-r.stopCh:
			return
		case ev := <-r.InputCh:
			r.applyInput(ev)
		case now := <-ticker.C:
			delta := now.Sub(last).Seconds()
			last = now

			r.updatePhysics(delta)
			r.emitStateSnapshot()
		}
	}
}

// Stop 结束房间循环。
func (r *BattleRoom) Stop() {
	select {
	case <-r.stopCh:
		// 已关闭，不重复 close
	default:
		close(r.stopCh)
	}
}

// applyInput 按 user_id 将最新输入写入玩家缓存。
func (r *BattleRoom) applyInput(ev InputEvent) {
	switch ev.UserID {
	case r.Player1.UserID:
		r.Player1.Input = ev.Input
	case r.Player2.UserID:
		r.Player2.Input = ev.Input
	}
}

// updatePhysics 执行每帧战斗物理推进。
func (r *BattleRoom) updatePhysics(delta float64) {
	// 修改内容：在每帧开始先批量消费 InputCh，避免输入滞后到下一帧
	// 修改原因：客户端输入可能在单帧内到达多条，需取最新意图推进权威状态
	// 影响范围：战斗输入响应速度与状态一致性
	for {
		select {
		case ev := <-r.InputCh:
			r.applyInput(ev)
		default:
			goto done
		}
	}
done:
	r.Player1.Update(delta)
	r.Player2.Update(delta)
	r.runCombatArbiter()
}

func (r *BattleRoom) runCombatArbiter() {
	p1 := &r.Player1
	p2 := &r.Player2

	p1MeleeHitP2 := r.checkMeleeHit(p1, p2)
	p2MeleeHitP1 := r.checkMeleeHit(p2, p1)
	p1DashHitP2 := r.checkDashHit(p1, p2)
	p2DashHitP1 := r.checkDashHit(p2, p1)

	// 情况 A【绝对拼刀】优先级最高。
	if (p1DashHitP2 && p2DashHitP1) ||
		(p1MeleeHitP2 && p2.CurrentState == Dashing) ||
		(p2MeleeHitP1 && p1.CurrentState == Dashing) {
		r.applyClash(p1, p2)
		r.enforceDeath(p1)
		r.enforceDeath(p2)
		return
	}

	// 情况 B【单方碾压】按顺序执行，首个命中会改变状态避免重复结算。
	if p1MeleeHitP2 || p1DashHitP2 {
		if p2.CurrentState != Dashing {
			r.applyNormalHit(p1, p2)
		}
	}
	if p2MeleeHitP1 || p2DashHitP1 {
		if p1.CurrentState != Dashing {
			r.applyNormalHit(p2, p1)
		}
	}

	r.enforceDeath(p1)
	r.enforceDeath(p2)
}

func (r *BattleRoom) checkMeleeHit(attacker, victim *BattlePlayer) bool {
	if attacker.CurrentState != Attack || attacker.HasHit {
		return false
	}
	dirToVictimRaw := victim.Position.Sub(attacker.Position)
	dist := dirToVictimRaw.Length()
	if dist > MeleeRadius || dist == 0 {
		return false
	}

	dirToVictim := dirToVictimRaw.Normalized()
	facingDir := Vector2{X: attacker.FacingX, Z: 0}
	return dirToVictim.Dot(facingDir) >= 0.5
}

func (r *BattleRoom) checkDashHit(attacker, victim *BattlePlayer) bool {
	if attacker.CurrentState != Dashing || attacker.HasHit {
		return false
	}
	dist := attacker.Position.Sub(victim.Position).Length()
	return dist <= DashHitRadius
}

func (r *BattleRoom) applyClash(p1, p2 *BattlePlayer) {
	p1.HP -= BaseDamage
	p2.HP -= BaseDamage

	p1.CurrentState = HitStun
	p2.CurrentState = HitStun
	p1.StateTimer = HitStunClash
	p2.StateTimer = HitStunClash

	// 拼刀弹开：双方向自己原本速度的反方向弹开
	push1 := p1.Velocity.Normalized().Mul(-1.0)
	push2 := p2.Velocity.Normalized().Mul(-1.0)
	if push1.Length() == 0 {
		push1 = Vector2{X: -p1.FacingX, Z: 0}
	}
	if push2.Length() == 0 {
		push2 = Vector2{X: -p2.FacingX, Z: 0}
	}
	p1.Velocity = push1.Mul(KnockbackSpeed)
	p2.Velocity = push2.Mul(KnockbackSpeed)

	p1.ChargeTimer = 0
	p2.ChargeTimer = 0

	p1.HasHit = true
	p2.HasHit = true
}

func (r *BattleRoom) applyNormalHit(attacker, victim *BattlePlayer) {
	attacker.Energy += EnergyReward
	if attacker.Energy > 15 {
		attacker.Energy = 15
	}
	attacker.CurrentState = PostCast
	attacker.StateTimer = AttackPostCastHit
	attacker.Velocity = Vector2{}
	attacker.HasHit = true

	victim.HP -= BaseDamage
	victim.CurrentState = HitStun
	victim.StateTimer = HitStunNormal
	victim.ChargeTimer = 0

	// 根据攻击方式决定绝对击退方向，防止穿模后坐标反转导致“吸人”
	var pushDir Vector2
	if attacker.CurrentState == Dashing {
		pushDir = attacker.Velocity.Normalized()
	} else {
		pushDir = Vector2{X: attacker.FacingX, Z: 0}
	}
	victim.Velocity = pushDir.Mul(KnockbackSpeed)
}

func (r *BattleRoom) enforceDeath(p *BattlePlayer) {
	if p.HP > 0 {
		return
	}
	p.HP = 0
	p.CurrentState = Dead
	p.StateTimer = 0
}

// emitStateSnapshot 将当前房间玩家状态组装为协议消息并投递到广播通道。
func (r *BattleRoom) emitStateSnapshot() {
	msg := &pb.GameMessage{
		Type:   "state",
		RoomId: r.RoomID,
		Players: []*pb.PlayerPos{
			{
				UserId:       r.Player1.UserID,
				X:            float32(r.Player1.Position.X),
				Y:            float32(r.Player1.Y),
				Z:            float32(r.Player1.Position.Z),
				RotY:         float32(r.Player1.RotY),
				CurrentState: uint32(r.Player1.CurrentState),
				Hp:           r.Player1.HP,
				Energy:       r.Player1.Energy,
			},
			{
				UserId:       r.Player2.UserID,
				X:            float32(r.Player2.Position.X),
				Y:            float32(r.Player2.Y),
				Z:            float32(r.Player2.Position.Z),
				RotY:         float32(r.Player2.RotY),
				CurrentState: uint32(r.Player2.CurrentState),
				Hp:           r.Player2.HP,
				Energy:       r.Player2.Energy,
			},
		},
	}

	payload, err := proto.Marshal(msg)
	if err != nil {
		return
	}

	// 非阻塞写入：避免消费者短暂阻塞时拖慢 60Hz 主循环。
	select {
	case r.BroadcastCh <- payload:
	default:
	}
}

// ===== 新增代码 END =====
