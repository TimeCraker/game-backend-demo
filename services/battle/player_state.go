package battle

import "math"

// ===== 新增代码 START =====
// 修改内容：定义战斗物理层常量参数（服务端权威推进的基础配置）
// 修改原因：统一服务端速度、蓄力、阻尼等关键参数，避免魔法数字散落
// 影响范围：battle 服务内的物理更新与状态结算（后续 updatePhysics 使用）
const (
	// 移动与物理参数
	BaseSpeed              = 200.0
	ChargeSpeedMultiplier  = 0.3
	MaxChargeTime          = 3.5
	MaxEffectiveChargeTime = 2.5
	AccelTime              = 1.33
	DashFriction           = 18.0
	HitStunFriction        = 8.0
	DashDistMultiplier     = 1.5

	// 战斗与状态时间参数 (秒)
	DashDuration           = 0.3 // 冲刺持续时间
	DashPostCast           = 0.5 // 冲刺结束后的后摇
	AttackPreCast          = 0.1 // 攻击前摇
	AttackDuration         = 0.05 // 攻击判定窗口
	AttackPostCastMiss     = 0.5 // 攻击挥空后摇惩罚
	AttackPostCastHit      = 0.3 // 攻击命中后摇奖励
	HitStunNormal          = 0.4 // 普通受击硬直
	HitStunClash           = 0.5 // 拼刀受击硬直

	// 判定与数值参数
	MeleeRadius            = 150.0 // 扇形攻击判定半径
	DashHitRadius          = 60.0  // 冲刺碰撞半径
	KnockbackSpeed         = 1600.0 // 击退初速度
	BaseDamage       int32 = 30    // 基础伤害
	EnergyReward     int32 = 1     // 命中回复能量
)

// State 表示玩家在战斗状态机中的离散状态枚举（与协议中的 current_state 对齐使用）
type State uint32

const (
	// Idle：静止待机状态
	Idle State = iota
	// Move：常规移动状态
	Move
	// Charging：蓄力状态
	Charging
	// Dashing：冲刺/突进状态
	Dashing
	// Attack：普通攻击状态
	Attack
	// PreCast：技能前摇状态
	PreCast
	// PostCast：技能后摇状态
	PostCast
	// HitStun：受击硬直状态
	HitStun
	// SkillCast：技能释放状态
	SkillCast
	// Dead：死亡状态
	Dead
)

// InputSnapshot 缓存最近一帧客户端输入，用于固定帧中推进权威逻辑。
type InputSnapshot struct {
	// InputX/InputY：归一化平面输入（通常来自摇杆或 WASD）
	InputX float64
	InputY float64
	// IsCharging/IsAttacking：动作按键状态
	IsCharging  bool
	IsAttacking bool
	// MouseX/MouseY：瞄准点或鼠标朝向输入
	MouseX float64
	MouseY float64
}

// BattlePlayer 表示战斗房间内由服务端权威管理的玩家运行时状态。
type BattlePlayer struct {
	// UserID：玩家唯一标识，对应协议 user_id
	UserID uint32
	// Position：XZ 平面权威位置（Y 轴独立保存）
	Position Vector2
	// Y：独立高度坐标（当前 2D 战斗可保持 0）
	Y float64
	// RotY：角色朝向（角度）
	RotY float64
	// Velocity：XZ 平面的瞬时速度
	Velocity Vector2
	// FacingX：水平朝向，仅存 1.0 或 -1.0
	FacingX float64
	// CurrentState：当前状态机状态（下行同步到 current_state）
	CurrentState State
	// HP/Energy：战斗核心属性（下行同步到 hp/energy）
	HP     int32
	Energy int32
	// StateTimer：状态机计时器（替代 Godot create_timer）
	StateTimer float64
	// ChargeTimer：当前蓄力累计时间（秒）
	ChargeTimer float64
	// HasHit：当前攻击/冲刺阶段是否已经触发过命中
	HasHit bool
	// Input：最近一次收到的输入快照
	Input InputSnapshot
	// ClassID：角色职业（如 Role1_Speedster），用于大招补丁分发
	ClassID string
	// SpeedsterBuffTimer：极速者大招 Buff 倒计时（蓄力加速、取消后摇、停能量等）
	SpeedsterBuffTimer float64
}

// Update 在单个 fixed-tick 内推进玩家状态机与物理。
func (p *BattlePlayer) Update(delta float64) {
	if p.SpeedsterBuffTimer > 0 {
		p.SpeedsterBuffTimer -= delta
	}
	// 修改内容：先推进计时状态机，严格复刻 PreCast->Attack->PostCast->Idle 时间链
	// 修改原因：Go 端无 create_timer，需要用显式倒计时保证服务端权威一致性
	// 影响范围：攻击前后摇、受击硬直等离散动作状态切换
	if p.StateTimer > 0 {
		p.StateTimer -= delta
	}
	if p.StateTimer <= 0 {
		switch p.CurrentState {
		case PreCast:
			p.CurrentState = Attack
			p.HasHit = false
			p.StateTimer = AttackDuration
		case Attack:
			if p.SpeedsterBuffTimer > 0 {
				p.CurrentState = Idle // 极速者大招：完全取消攻击后摇
				p.StateTimer = 0
			} else {
				p.CurrentState = PostCast
				if p.HasHit {
					p.StateTimer = AttackPostCastHit
				} else {
					p.StateTimer = AttackPostCastMiss
				}
			}
		case Dashing:
			if p.SpeedsterBuffTimer > 0 {
				p.CurrentState = Idle // 极速者大招：完全取消冲刺后摇
				p.StateTimer = 0
			} else {
				p.CurrentState = PostCast
				p.StateTimer = DashPostCast
			}
		case PostCast, HitStun, SkillCast:
			p.CurrentState = Idle
			p.StateTimer = 0
		}
	}

	// 由输入触发攻击（仅可移动状态可进入前摇）。
	if p.Input.IsAttacking && (p.CurrentState == Idle || p.CurrentState == Move || p.CurrentState == Charging) {
		p.CurrentState = PreCast
		p.StateTimer = AttackPreCast
	}

	inputVec := Vector2{X: p.Input.InputX, Z: p.Input.InputY}
	inputDir := inputVec.Normalized()

	if p.Input.IsCharging {
		dx := p.Input.MouseX - p.Position.X
		if dx > 0 {
			p.FacingX = 1.0
		} else if dx < 0 {
			p.FacingX = -1.0
		}
	}

	// 蓄力状态机：按住进入 Charging，松开从 Charging 进入 Dashing。
	if p.CurrentState == Idle || p.CurrentState == Move || p.CurrentState == Charging {
		if p.Input.IsCharging {
			p.CurrentState = Charging
			chargeRate := 1.0
			if p.SpeedsterBuffTimer > 0 {
				chargeRate = 1.5 // 极速者大招期间蓄力速度提升 1.5 倍
			}
			p.ChargeTimer += delta * chargeRate
			if p.ChargeTimer >= MaxChargeTime {
				p.startDashFromCharge()
			}
		} else if p.CurrentState == Charging {
			p.startDashFromCharge()
		} else if inputDir.Length() > 0 {
			p.CurrentState = Move
		} else {
			p.CurrentState = Idle
		}
	}

	if !p.Input.IsCharging && p.CurrentState == Move && p.Input.InputX != 0 {
		if p.Input.InputX > 0 {
			p.FacingX = 1.0
		} else {
			p.FacingX = -1.0
		}
	}

	// 非可移动状态不吃普通位移输入。
	if !(p.CurrentState == Idle || p.CurrentState == Move || p.CurrentState == Charging || p.CurrentState == Dashing) {
		inputDir = Vector2{}
	}

	// 物理三轨制：
	// 轨道 A1：受击与死亡（台球级顺滑刹车）
	if p.CurrentState == HitStun || p.CurrentState == Dead {
		p.Velocity = p.Velocity.Lerp(Vector2{}, HitStunFriction*delta)
	} else if p.CurrentState == SkillCast {
		p.Velocity = p.Velocity.Lerp(Vector2{}, DashFriction*delta)
	} else if p.CurrentState == Dashing {
		// 轨道 A2：冲刺（极速急停）；结束时机由 StateTimer 驱动，见 Dashing 分支
		p.Velocity = p.Velocity.Lerp(Vector2{}, DashFriction*delta)
	} else {
		currentMaxSpeed := BaseSpeed
		if p.CurrentState == Charging {
			currentMaxSpeed = BaseSpeed * ChargeSpeedMultiplier
		}

		targetVelocity := inputDir.Mul(currentMaxSpeed)
		accelStep := (BaseSpeed / AccelTime) * delta

		var linearTarget Vector2
		if inputDir.Length() == 0 {
			linearTarget = p.Velocity.MoveToward(Vector2{}, accelStep)
		} else {
			if p.Velocity.Length() > 0 && inputDir.Dot(p.Velocity.Normalized()) < 0 {
				// 输入反向时采用 4 倍制动，加快掉头响应（对标 Godot 逻辑）。
				accelStep *= 4.0
			}
			linearTarget = p.Velocity.MoveToward(targetVelocity, accelStep)
		}
		p.Velocity = p.Velocity.Lerp(linearTarget, 25.0*delta)
	}

	// 应用位移积分（XZ 平面）。
	p.Position.X += p.Velocity.X * delta
	p.Position.Z += p.Velocity.Z * delta

	// 使用输入鼠标向量更新朝向（与客户端鼠标控制语义保持一致）。
	dirToMouse := Vector2{X: p.Input.MouseX - p.Position.X, Z: p.Input.MouseY - p.Position.Z}
	if dirToMouse.Length() > 0.001 {
		p.RotY = math.Atan2(dirToMouse.Z, dirToMouse.X) * 180.0 / math.Pi
	}

}

// startDashFromCharge 按蓄力时间转换冲刺初速度。
func (p *BattlePlayer) startDashFromCharge() {
	effectiveTime := math.Min(p.ChargeTimer, MaxEffectiveChargeTime)
	distance := effectiveTime * (BaseSpeed * DashDistMultiplier)
	p.ChargeTimer = 0

	if distance < 10 {
		p.CurrentState = Idle
		return
	}

	p.CurrentState = Dashing
	p.StateTimer = DashDuration
	p.HasHit = false

	// 【核心修复】：恢复 360 度鼠标瞄准冲刺
	dirToMouse := Vector2{X: p.Input.MouseX - p.Position.X, Z: p.Input.MouseY - p.Position.Z}
	dir := dirToMouse.Normalized()
	if dir.Length() == 0 {
		dir = Vector2{X: p.FacingX, Z: 0} // 兜底保护
	}
	initialBurstSpeed := distance * DashFriction
	p.Velocity = p.Velocity.Add(dir.Mul(initialBurstSpeed))

	// 极速者大招期间停止积攒能量
	if p.SpeedsterBuffTimer <= 0 {
		energyGain := int32(effectiveTime * 2.0)
		p.Energy += energyGain
		if p.Energy > 15 {
			p.Energy = 15
		}
	}
}

// ===== 新增代码 END =====
