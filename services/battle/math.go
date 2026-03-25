package battle

import "math"

// ===== 新增代码 START =====
// 修改内容：定义战斗平面 Vector2 数学结构与基础运算
// 修改原因：服务端需要与 Godot 侧一致的向量运算语义来复刻移动/刹车逻辑
// 影响范围：battle 玩家物理推进、插值与速度逼近算法
type Vector2 struct {
	X float64
	Z float64
}

// Add 向量加法。
func (v Vector2) Add(other Vector2) Vector2 {
	return Vector2{X: v.X + other.X, Z: v.Z + other.Z}
}

// Sub 向量减法。
func (v Vector2) Sub(other Vector2) Vector2 {
	return Vector2{X: v.X - other.X, Z: v.Z - other.Z}
}

// Mul 标量乘法。
func (v Vector2) Mul(scalar float64) Vector2 {
	return Vector2{X: v.X * scalar, Z: v.Z * scalar}
}

// Length 返回向量长度。
func (v Vector2) Length() float64 {
	return math.Sqrt(v.X*v.X + v.Z*v.Z)
}

// Normalized 返回归一化向量（零向量返回零向量）。
func (v Vector2) Normalized() Vector2 {
	l := v.Length()
	if l <= 0 {
		return Vector2{}
	}
	return Vector2{X: v.X / l, Z: v.Z / l}
}

// Dot 返回点积。
func (v Vector2) Dot(other Vector2) float64 {
	return v.X*other.X + v.Z*other.Z
}

// Lerp 线性插值（对标 Godot 的 v + (target - v) * weight）。
func (v Vector2) Lerp(target Vector2, weight float64) Vector2 {
	return Vector2{
		X: v.X + (target.X-v.X)*weight,
		Z: v.Z + (target.Z-v.Z)*weight,
	}
}

// MoveToward 向目标最多移动 maxDelta（对标 Godot move_toward）。
func (v Vector2) MoveToward(target Vector2, maxDelta float64) Vector2 {
	delta := target.Sub(v)
	dist := delta.Length()
	if dist <= maxDelta || dist == 0 {
		return target
	}
	return v.Add(delta.Mul(maxDelta / dist))
}

// ===== 新增代码 END =====
