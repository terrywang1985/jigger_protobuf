package main_bak

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"math"
	"os"
	pb "proto"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// 异步日志示例（可选）
var logCh = make(chan string, 1000)

func init() {
	go func() {
		for msg := range logCh {
			log.Print(msg)
		}
	}()
}

func asyncLog(format string, v ...interface{}) {
	select {
	case logCh <- fmt.Sprintf(format, v...):
	default:
		log.Printf(format, v...)
	}
}

// --------------------
// 向量辅助函数及类型
// --------------------
type Vector2 struct {
	X, Y float64
}

func vecLength(v Vector2) float64 {
	return math.Sqrt(v.X*v.X + v.Y*v.Y)
}

func vecNormalize(v Vector2) Vector2 {
	l := vecLength(v)
	if l == 0 {
		return Vector2{0, 0}
	}
	return Vector2{v.X / l, v.Y / l}
}

func vecAdd(v1, v2 Vector2) Vector2 {
	return Vector2{v1.X + v2.X, v1.Y + v2.Y}
}

func vecSub(v1, v2 Vector2) Vector2 {
	return Vector2{v1.X - v2.X, v1.Y - v2.Y}
}

func vecScale(v Vector2, s float64) Vector2 {
	return Vector2{v.X * s, v.Y * s}
}

// rotateVector 以角度（单位：度）旋转向量
func rotateVector(v Vector2, angle float64) Vector2 {
	rad := angle * math.Pi / 180
	return Vector2{
		X: v.X*math.Cos(rad) - v.Y*math.Sin(rad),
		Y: v.X*math.Sin(rad) + v.Y*math.Cos(rad),
	}
}

// --------------------
// 原子时间封装：解决 time.Time 对齐问题
// --------------------
type atomicTime struct {
	v atomic.Int64
}

func (a *atomicTime) Store(t time.Time) {
	a.v.Store(t.UnixNano())
}

func (a *atomicTime) Load() time.Time {
	return time.Unix(0, a.v.Load())
}

// --------------------
// ECS 基础定义
// --------------------
type Entity uint64

// 单位类型（与 proto 中 EntityType 对应）
const (
	UnitCard    = 1
	UnitTower   = 2
	UnitCrystal = 3
)

// --------------------
// 碰撞相关定义（均采用圆形）
//
// 对于卡牌，我们选取半径20；水晶与塔使用各自定义的半径
// --------------------
const (
	CollisionCircle = 1
)

type CollisionComponent struct {
	ShapeType int     // 1: 圆形
	Radius    float64 // 圆形半径
}

// --------------------
// 组件定义
// --------------------
type PositionComponent struct {
	X, Y float64
}

type HPComponent struct {
	HP float64
}

type AttackComponent struct {
	Attack      float64       // 基础伤害
	AttackSpeed time.Duration // 攻击间隔
	LastAttack  time.Time     // 上次攻击时间
	AttackRange float64       // 攻击范围
	MP          float64       // 当前魔法值
	MPCost      float64       // 攻击消耗 MP
	MPRegen     float64       // 每秒回复 MP
}

type MovementComponent struct {
	MoveSpeed     float64
	VelocityCache Vector2 // 当前速度
}

type TargetComponent struct {
	Target Entity
}

type OwnerComponent struct {
	PlayerID uint64
}

type CardInfoComponent struct {
	CardID   uint64
	UnitType int // 对应 proto 中的枚举
}

// Skill 结构使用 atomicTime 存储 LastUsed
type Skill struct {
	Skill    pb.MagicSkill
	LastUsed atomicTime
}

type SkillComponent struct {
	Skills map[uint64]*Skill
}

type GrowthComponent struct {
	Level int
	XP    int
}

type EffectComponent struct {
	EffectType int
	Value      float64
	Duration   time.Duration
	StartTime  time.Time
	Priority   int
}

const (
	EFFECT_SLOW   = 1
	EFFECT_STUN   = 2
	EFFECT_FREEZE = 3
)

// --------------------
// 事件系统定义
// --------------------
type EventType int

const (
	EntitySpawn EventType = iota
	EntityDeath
	SkillCast
)

type DeathEvent struct {
	Entity Entity
	Killer Entity
}

type EventBus struct {
	sync.RWMutex
	subscribers map[EventType][]func(interface{})
}

func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[EventType][]func(interface{})),
	}
}

func (bus *EventBus) Subscribe(eventType EventType, handler func(interface{})) {
	bus.Lock()
	defer bus.Unlock()
	bus.subscribers[eventType] = append(bus.subscribers[eventType], handler)
}

func (bus *EventBus) Publish(eventType EventType, event interface{}) {
	bus.RLock()
	defer bus.RUnlock()
	if handlers, ok := bus.subscribers[eventType]; ok {
		for _, handler := range handlers {
			go handler(event)
		}
	}
}

var eventBus = NewEventBus()

// --------------------
// 配置驱动技能：SkillConfig
// --------------------
type SkillConfig struct {
	SkillID uint64                 `json:"skill_id"`
	Name    string                 `json:"name"`
	Handler string                 `json:"handler"`
	Params  map[string]interface{} `json:"params"`
}

var skillConfigs map[uint64]SkillConfig

//func loadSkillConfigs() {
//	//jsonStr := `[
//	//	{
//	//		"skill_id": 201,
//	//		"name": "火球术",
//	//		"handler": "FireballHandler",
//	//		"params": {
//	//			"radius": 5.0,
//	//			"damage_multiplier": 1.2
//	//		}
//	//	}
//	//]`
//	//var configs []SkillConfig
//	//err := json.Unmarshal([]byte(jsonStr), &configs)
//	//if err != nil {
//	//	log.Printf("Failed to load skill configs: %v", err)
//	//	return
//	//}
//	skillConfigs = make(map[uint64]SkillConfig)
//	//for _, config := range configs {
//	//	skillConfigs[config.SkillID] = config
//	//}
}

// --------------------
// ECSWorld：存储所有组件
// --------------------
type ECSWorld struct {
	sync.RWMutex
	nextEntityID   Entity
	pendingRemoval []Entity

	positions  map[Entity]*PositionComponent
	hps        map[Entity]*HPComponent
	attacks    map[Entity]*AttackComponent
	movements  map[Entity]*MovementComponent
	targets    map[Entity]*TargetComponent
	owners     map[Entity]*OwnerComponent
	cardInfos  map[Entity]*CardInfoComponent
	skills     map[Entity]*SkillComponent
	growth     map[Entity]*GrowthComponent
	effects    map[Entity][]*EffectComponent
	collisions map[Entity]*CollisionComponent
}

func NewECSWorld() *ECSWorld {
	return &ECSWorld{
		nextEntityID: 1,
		positions:    make(map[Entity]*PositionComponent),
		hps:          make(map[Entity]*HPComponent),
		attacks:      make(map[Entity]*AttackComponent),
		movements:    make(map[Entity]*MovementComponent),
		targets:      make(map[Entity]*TargetComponent),
		owners:       make(map[Entity]*OwnerComponent),
		cardInfos:    make(map[Entity]*CardInfoComponent),
		skills:       make(map[Entity]*SkillComponent),
		growth:       make(map[Entity]*GrowthComponent),
		effects:      make(map[Entity][]*EffectComponent),
		collisions:   make(map[Entity]*CollisionComponent),
	}
}

func (w *ECSWorld) NewEntity() Entity {
	w.Lock()
	defer w.Unlock()
	e := w.nextEntityID
	w.nextEntityID++
	return e
}

func (w *ECSWorld) removeEntity(e Entity) {
	delete(w.positions, e)
	delete(w.hps, e)
	delete(w.attacks, e)
	delete(w.movements, e)
	delete(w.targets, e)
	delete(w.owners, e)
	delete(w.cardInfos, e)
	delete(w.skills, e)
	delete(w.growth, e)
	delete(w.effects, e)
	delete(w.collisions, e)
}

func (w *ECSWorld) CleanupEntities() {
	w.Lock()
	defer w.Unlock()
	for _, e := range w.pendingRemoval {
		w.removeEntity(e)
	}
	w.pendingRemoval = nil
}

// --------------------
// 辅助：位置合法性检查（保证在 [0,1000]×[0,600] 内）
// --------------------
func isValidPosition(x, y float64) bool {
	return x >= 0 && x <= 1000 && y >= 0 && y <= 600
}

// --------------------
// 碰撞检测：仅处理圆形碰撞
// --------------------
func circleCircleCollision(x1, y1, r1, x2, y2, r2 float64) bool {
	dx := x1 - x2
	dy := y1 - y2
	return dx*dx+dy*dy < (r1+r2)*(r1+r2)
}

func collisionBetweenFloat(x1, y1 float64, col1 *CollisionComponent, x2, y2 float64, col2 *CollisionComponent) bool {
	if col1.ShapeType == CollisionCircle && col2.ShapeType == CollisionCircle {
		return circleCircleCollision(x1, y1, col1.Radius, x2, y2, col2.Radius)
	}
	return false
}

func (w *ECSWorld) CheckCollision(e Entity, newX, newY float64) bool {
	col, ok := w.collisions[e]
	if !ok {
		return false
	}
	for other, oCol := range w.collisions {
		if other == e {
			continue
		}
		otherPos, ok := w.positions[other]
		if !ok {
			continue
		}
		if collisionBetweenFloat(newX, newY, col, otherPos.X, otherPos.Y, oCol) {
			return true
		}
	}
	return false
}

func (w *ECSWorld) GetCollisionPenetration(e Entity, testX, testY float64) float64 {
	col, ok := w.collisions[e]
	if !ok {
		return 0
	}
	minPenetration := math.MaxFloat64
	for other, oCol := range w.collisions {
		if other == e {
			continue
		}
		otherPos, ok := w.positions[other]
		if !ok {
			continue
		}
		if collisionBetweenFloat(testX, testY, col, otherPos.X, otherPos.Y, oCol) {
			dx := testX - otherPos.X
			dy := testY - otherPos.Y
			distance := math.Sqrt(dx*dx + dy*dy)
			allowed := col.Radius + oCol.Radius
			penetration := allowed - distance
			if penetration < minPenetration {
				minPenetration = penetration
			}
		}
	}
	if minPenetration == math.MaxFloat64 {
		return 0
	}
	return minPenetration
}

// --------------------
// 效果处理
// --------------------
func (w *ECSWorld) ApplyEffect(target Entity, effectType int, value float64, duration time.Duration, now time.Time) {
	w.Lock()
	w.effects[target] = append(w.effects[target], &EffectComponent{
		EffectType: effectType,
		Value:      value,
		Duration:   duration,
		StartTime:  now,
		Priority:   1,
	})
	w.Unlock()
	log.Printf("Entity %d gets effect type %d with value %.2f for %v", target, effectType, value, duration)
}

func (w *ECSWorld) CleanupExpiredEffects(now time.Time) {
	w.Lock()
	defer w.Unlock()
	for e, effs := range w.effects {
		valid := effs[:0]
		for _, eff := range effs {
			if now.Sub(eff.StartTime) <= eff.Duration {
				valid = append(valid, eff)
			}
		}
		w.effects[e] = valid
	}
}

func (w *ECSWorld) GetEffectiveMoveSpeed(e Entity, baseSpeed float64, now time.Time) float64 {
	var activeEffects []*EffectComponent
	if effs, ok := w.effects[e]; ok {
		for _, eff := range effs {
			if now.Sub(eff.StartTime) <= eff.Duration {
				activeEffects = append(activeEffects, eff)
			}
		}
	}
	if len(activeEffects) == 0 {
		return baseSpeed
	}
	sort.Slice(activeEffects, func(i, j int) bool {
		return activeEffects[i].Priority > activeEffects[j].Priority
	})
	if activeEffects[0].EffectType == EFFECT_FREEZE {
		return 0
	}
	totalSlow := 0.0
	for _, eff := range activeEffects {
		if eff.EffectType == EFFECT_SLOW {
			totalSlow += eff.Value
		}
	}
	if totalSlow > 1 {
		totalSlow = 1
	}
	return baseSpeed * (1 - totalSlow)
}

func (w *ECSWorld) ApplyDamage(target Entity, damage float64, attacker Entity) {
	w.Lock()
	defer w.Unlock()
	if hp, ok := w.hps[target]; ok {
		hp.HP -= damage
		if hp.HP <= 0 {
			w.pendingRemoval = append(w.pendingRemoval, target)
			log.Printf("Entity %d marked for removal (HP <= 0)", target)
			eventBus.Publish(EntityDeath, DeathEvent{Entity: target, Killer: attacker})
		}
	}
}

// --------------------
// 技能触发条件检查
// --------------------
func (w *ECSWorld) CanCastSkill(e Entity, s Skill, target Entity) bool {
	w.RLock()
	defer w.RUnlock()
	pos, ok := w.positions[e]
	if !ok || pos == nil {
		log.Printf("Entity %d missing position", e)
		return false
	}
	tgtPos, ok := w.positions[target]
	if !ok || tgtPos == nil {
		log.Printf("Target %d missing position", target)
		return false
	}
	if tgtHP, ok := w.hps[target]; !ok || tgtHP.HP <= 0 {
		return false
	}
	switch pb.MagicTargetType(s.Skill.TargetType) {
	case pb.MagicTargetType_ENEMY:
		ownerE := w.owners[e]
		ownerT := w.owners[target]
		if ownerE != nil && ownerT != nil && ownerE.PlayerID == ownerT.PlayerID {
			return false
		}
	case pb.MagicTargetType_FRIEND:
		ownerE := w.owners[e]
		ownerT := w.owners[target]
		if ownerE != nil && ownerT != nil && ownerE.PlayerID != ownerT.PlayerID {
			return false
		}
	case pb.MagicTargetType_SELF:
		if e != target {
			return false
		}
	}
	dx := pos.X - tgtPos.X
	dy := pos.Y - tgtPos.Y
	return math.Sqrt(dx*dx+dy*dy) <= float64(s.Skill.RangeRadius)
}

// --------------------
// 战场初始化
// --------------------
func (w *ECSWorld) InitBattlefield() {
	// 玩家1：水晶与塔
	p1Crystal := w.NewEntity()
	w.positions[p1Crystal] = &PositionComponent{X: 500, Y: 50}
	w.hps[p1Crystal] = &HPComponent{HP: 2000}
	w.owners[p1Crystal] = &OwnerComponent{PlayerID: 1}
	w.cardInfos[p1Crystal] = &CardInfoComponent{CardID: 0, UnitType: UnitCrystal}
	w.collisions[p1Crystal] = &CollisionComponent{ShapeType: CollisionCircle, Radius: 40}

	slog.Info("Player 1 Crystal", "entity", p1Crystal, "position", w.positions[p1Crystal])

	towerPositions := []struct{ x, y float64 }{{300, 150}, {700, 150}}
	for _, tp := range towerPositions {
		t := w.NewEntity()
		w.positions[t] = &PositionComponent{X: tp.x, Y: tp.y}
		w.hps[t] = &HPComponent{HP: 1000}
		w.attacks[t] = &AttackComponent{
			Attack:      50,
			AttackSpeed: 2 * time.Second,
			LastAttack:  time.Now(),
			AttackRange: 120,
			MP:          0,
			MPCost:      0,
			MPRegen:     0,
		}
		w.owners[t] = &OwnerComponent{PlayerID: 1}
		w.cardInfos[t] = &CardInfoComponent{CardID: 0, UnitType: UnitTower}
		w.collisions[t] = &CollisionComponent{ShapeType: CollisionCircle, Radius: 20}

		slog.Info("Player 1 Tower", "entity", t, "position", w.positions[t])
	}

	// 玩家2：水晶与塔
	p2Crystal := w.NewEntity()
	w.positions[p2Crystal] = &PositionComponent{X: 500, Y: 550}
	w.hps[p2Crystal] = &HPComponent{HP: 2000}
	w.owners[p2Crystal] = &OwnerComponent{PlayerID: 2}
	w.cardInfos[p2Crystal] = &CardInfoComponent{CardID: 0, UnitType: UnitCrystal}
	w.collisions[p2Crystal] = &CollisionComponent{ShapeType: CollisionCircle, Radius: 40}

	slog.Info("Player 2 Crystal", "entity", p2Crystal, "position", w.positions[p2Crystal])

	towerPositions2 := []struct{ x, y float64 }{{300, 450}, {700, 450}}
	for _, tp := range towerPositions2 {
		t := w.NewEntity()
		w.positions[t] = &PositionComponent{X: tp.x, Y: tp.y}
		w.hps[t] = &HPComponent{HP: 1000}
		w.attacks[t] = &AttackComponent{
			Attack:      50,
			AttackSpeed: 2 * time.Second,
			LastAttack:  time.Now(),
			AttackRange: 120,
			MP:          0,
			MPCost:      0,
			MPRegen:     0,
		}
		w.owners[t] = &OwnerComponent{PlayerID: 2}
		w.cardInfos[t] = &CardInfoComponent{CardID: 0, UnitType: UnitTower}
		w.collisions[t] = &CollisionComponent{ShapeType: CollisionCircle, Radius: 20}

		slog.Info("Player 2 Tower", "entity", t, "position", w.positions[t])
	}
}

// --------------------
// ProcessPlaceCardAction：处理玩家下卡牌
//
// 修改点：加入对站位区域的限制，竖屏情况下：
// - 玩家1只能放置在上半边（y < 300）
// - 玩家2只能放置在下半边（y >= 300）
// --------------------
func (w *ECSWorld) ProcessPlaceCardAction(action *pb.PlaceCardAction, card pb.Card, ownerID uint64) {
	posX := float64(action.Pos.X)
	posY := float64(action.Pos.Y)
	// 基础位置合法性检查
	if !isValidPosition(posX, posY) {
		log.Printf("Invalid position: (%d, %d)", action.Pos.X, action.Pos.Y)
		return
	}
	// 站位区域限制
	if ownerID == 1 && posY >= 300 {
		log.Printf("Player 1 cannot place card in opponent area: y=%d", action.Pos.Y)
		return
	}
	if ownerID == 2 && posY < 300 {
		log.Printf("Player 2 cannot place card in opponent area: y=%d", action.Pos.Y)
		return
	}
	e := w.NewEntity()
	w.Lock()
	w.positions[e] = &PositionComponent{X: posX, Y: posY}
	w.hps[e] = &HPComponent{HP: float64(card.Hp)}
	w.attacks[e] = &AttackComponent{
		Attack:      float64(card.Attack),
		AttackSpeed: time.Duration(card.AttackSpeed) * time.Millisecond,
		LastAttack:  time.Now(),
		AttackRange: float64(card.AttackRange),
		MP:          100,
		MPCost:      10,
		MPRegen:     1,
	}
	w.movements[e] = &MovementComponent{MoveSpeed: float64(card.MoveSpeed)}
	w.targets[e] = &TargetComponent{Target: 0}
	w.cardInfos[e] = &CardInfoComponent{CardID: card.Id, UnitType: UnitCard}
	w.owners[e] = &OwnerComponent{PlayerID: ownerID}
	if len(card.MagicSkill) > 0 {
		sComp := &SkillComponent{Skills: make(map[uint64]*Skill)}
		for _, ms := range card.MagicSkill {
			s := &Skill{Skill: *ms}
			s.LastUsed.Store(time.Now().Add(-time.Duration(ms.Cooldown) * time.Millisecond))
			sComp.Skills[ms.Id] = s
		}
		w.skills[e] = sComp
	}
	w.growth[e] = &GrowthComponent{Level: int(card.Growth.Level), XP: 0}
	// 卡牌采用圆形碰撞，半径20
	w.collisions[e] = &CollisionComponent{ShapeType: CollisionCircle, Radius: 20}
	w.Unlock()
	log.Printf("[Server] PlaceCard: card %d placed as entity %d at (%d, %d) for player %d",
		card.Id, e, action.Pos.X, action.Pos.Y, ownerID)
	eventBus.Publish(EntitySpawn, e)
}

// --------------------
// 技能处理——可插拔技能体系
// --------------------
type SkillHandler interface {
	Execute(world *ECSWorld, caster Entity, target Entity, skill pb.MagicSkill, now time.Time)
}

var skillRegistry = make(map[string]SkillHandler)

func RegisterSkill(name string, handler SkillHandler) {
	skillRegistry[name] = handler
}

type FireballHandler struct{}

func (f *FireballHandler) Execute(world *ECSWorld, caster Entity, target Entity, skill pb.MagicSkill, now time.Time) {
	radius := float64(skill.RangeRadius)
	damage := float64(skill.MagicPower)
	if config, ok := skillConfigs[skill.Id]; ok {
		if r, ok := config.Params["radius"].(float64); ok {
			radius = r
		}
		if dm, ok := config.Params["damage_multiplier"].(float64); ok {
			damage *= dm
		}
	}
	world.RLock()
	tPos, ok := world.positions[target]
	if !ok {
		world.RUnlock()
		log.Printf("Fireball: target %d has no position", target)
		return
	}
	positions := make([]struct {
		e   Entity
		pos PositionComponent
	}, 0, len(world.positions))
	for e, pos := range world.positions {
		positions = append(positions, struct {
			e   Entity
			pos PositionComponent
		}{e, *pos})
	}
	world.RUnlock()
	var targets []Entity
	for _, item := range positions {
		dx := item.pos.X - tPos.X
		dy := item.pos.Y - tPos.Y
		if math.Sqrt(dx*dx+dy*dy) <= radius {
			targets = append(targets, item.e)
		}
	}
	for _, other := range targets {
		world.ApplyDamage(other, damage, caster)
		log.Printf("Fireball: Entity %d damages entity %d for %.1f damage", caster, other, damage)
	}
}

type FrostNovaHandler struct{}

func (f *FrostNovaHandler) Execute(world *ECSWorld, caster Entity, target Entity, skill pb.MagicSkill, now time.Time) {
	damage := float64(skill.MagicPower)
	world.Lock()
	if tgtHP, ok := world.hps[target]; ok {
		tgtHP.HP -= damage
		log.Printf("FrostNova: Entity %d damages target %d for %.1f damage", caster, target, damage)
	}
	world.Unlock()
	world.ApplyEffect(target, EFFECT_SLOW, 0.3, 2*time.Second, now)
}

type IceArrowHandler struct{}

func (i *IceArrowHandler) Execute(world *ECSWorld, caster Entity, target Entity, skill pb.MagicSkill, now time.Time) {
	damage := float64(skill.MagicPower)
	world.Lock()
	if tgtHP, ok := world.hps[target]; ok {
		tgtHP.HP -= damage
		log.Printf("IceArrow: Entity %d damages target %d for %.1f damage", caster, target, damage)
	}
	world.Unlock()
	world.ApplyEffect(target, EFFECT_FREEZE, 1.0, 1500*time.Millisecond, now)
}

type HealHandler struct{}

func (h *HealHandler) Execute(world *ECSWorld, caster Entity, target Entity, skill pb.MagicSkill, now time.Time) {
	world.Lock()
	if hp, ok := world.hps[caster]; ok {
		healValue := 20.0
		hp.HP += healValue
		log.Printf("Heal: Entity %d heals itself for %.1f HP", caster, healValue)
	}
	world.Unlock()
}

func initSkillRegistry() {
	//loadSkillConfigs()
	RegisterSkill("火球术", &FireballHandler{})
	RegisterSkill("霜冻新星", &FrostNovaHandler{})
	RegisterSkill("冰箭", &IceArrowHandler{})
	RegisterSkill("治疗术", &HealHandler{})
}

// ProcessSkills：传入目标快照以避免数据竞争
func (w *ECSWorld) ProcessSkills(e Entity, now time.Time, targetSnapshot Entity) {
	var skillsToProcess map[uint64]*Skill
	w.Lock()
	if sComp, exists := w.skills[e]; exists {
		skillsToProcess = make(map[uint64]*Skill, len(sComp.Skills))
		for k, v := range sComp.Skills {
			skillsToProcess[k] = v
		}
	}
	w.Unlock()
	for _, s := range skillsToProcess {
		lastUsed := s.LastUsed.Load()
		if now.Sub(lastUsed) < time.Duration(s.Skill.Cooldown)*time.Millisecond {
			continue
		}
		if !w.CanCastSkill(e, *s, targetSnapshot) {
			continue
		}
		if handler, ok := skillRegistry[s.Skill.Name]; ok {
			handler.Execute(w, e, targetSnapshot, s.Skill, now)
			s.LastUsed.Store(now)
			eventBus.Publish(SkillCast, fmt.Sprintf("Entity %d cast skill %s", e, s.Skill.Name))
		}
	}
}

// --------------------
// UpdateTargeting：查找最近的敌对单位
// --------------------
func (w *ECSWorld) UpdateTargeting() {
	w.Lock()
	defer w.Unlock()
	for e := range w.attacks {
		tgtComp, exists := w.targets[e]
		if !exists {
			continue
		}
		if tgtComp.Target != 0 {
			if hp, ok := w.hps[tgtComp.Target]; !ok || hp.HP <= 0 {
				tgtComp.Target = 0
			} else {
				ownerE := w.owners[e]
				ownerT := w.owners[tgtComp.Target]
				if ownerE != nil && ownerT != nil && ownerE.PlayerID == ownerT.PlayerID {
					tgtComp.Target = 0
				} else {
					continue
				}
			}
		}
		pos, ok := w.positions[e]
		if !ok || pos == nil {
			log.Printf("UpdateTargeting: Entity %d missing position", e)
			continue
		}
		minDist := math.MaxFloat64
		var bestTarget Entity = 0
		ownerE := w.owners[e]
		for other, otherHP := range w.hps {
			if other == e || otherHP.HP <= 0 {
				continue
			}
			ownerOther := w.owners[other]
			if ownerE != nil && ownerOther != nil && ownerE.PlayerID == ownerOther.PlayerID {
				continue
			}
			otherPos, ok := w.positions[other]
			if !ok || otherPos == nil {
				continue
			}
			dx := otherPos.X - pos.X
			dy := otherPos.Y - pos.Y
			dist := math.Sqrt(dx*dx + dy*dy)
			if dist < minDist {
				minDist = dist
				bestTarget = other
			}
		}
		if bestTarget != 0 {
			w.targets[e].Target = bestTarget
			log.Printf("Entity %d targets enemy entity %d", e, bestTarget)
		}
	}
}

// --------------------
// UpdateMovement：更新移动，加入平滑、动态安全距离、左右偏转与边界处理
// --------------------
func (w *ECSWorld) UpdateMovement(dt time.Duration, now time.Time) {
	const smoothFactor = 0.4
	w.Lock()
	seconds := dt.Seconds()
	for e, mov := range w.movements {
		tgtComp, exists := w.targets[e]
		if !exists || tgtComp.Target == 0 {
			mov.VelocityCache = Vector2{0, 0}
			continue
		}
		pos, ok := w.positions[e]
		if !ok || pos == nil {
			continue
		}
		tgtPos, ok := w.positions[tgtComp.Target]
		if !ok || tgtPos == nil {
			continue
		}
		currentDir := Vector2{0, 0}
		if vecLength(mov.VelocityCache) > 0 {
			currentDir = vecNormalize(mov.VelocityCache)
		}
		desired := vecNormalize(vecSub(Vector2{tgtPos.X, tgtPos.Y}, Vector2{pos.X, pos.Y}))
		currentSpeed := vecLength(mov.VelocityCache)
		safeDistance := math.Max(30, currentSpeed*0.5)
		avoidance := Vector2{0, 0}
		for other, otherPos := range w.positions {
			if other == e {
				continue
			}
			diff := vecSub(Vector2{pos.X, pos.Y}, Vector2{otherPos.X, otherPos.Y})
			dist := vecLength(diff)
			if dist < safeDistance && dist > 0 {
				weight := ((safeDistance - dist) / safeDistance) * 2.0
				avoidance = vecAdd(avoidance, vecScale(vecNormalize(diff), weight))
			}
		}
		//combined := vecAdd(desired, avoidance)
		avoidanceWeight := 0.1 // 降低避障权重
		scaledAvoidance := vecScale(avoidance, avoidanceWeight)
		combined := vecAdd(vecScale(desired, 0.9), scaledAvoidance) // 增加原始方向的权重
		if vecLength(combined) < 1e-3 {
			combined = rotateVector(desired, 45)
		}
		newDir := vecNormalize(vecAdd(vecScale(currentDir, 1-smoothFactor), vecScale(combined, smoothFactor)))
		actualSpeed := w.GetEffectiveMoveSpeed(e, mov.MoveSpeed, now)
		moveDist := actualSpeed * seconds
		bestDir := newDir
		minPenetration := math.MaxFloat64
		angles := []float64{0, -15, 15, -30, 30, -45, 45}
		for _, angle := range angles {
			testDir := rotateVector(newDir, angle)
			testPos := vecAdd(Vector2{pos.X, pos.Y}, vecScale(testDir, moveDist))
			penetration := w.GetCollisionPenetration(e, testPos.X, testPos.Y)
			if penetration < minPenetration {
				minPenetration = penetration
				bestDir = testDir
			}
		}
		proposedX := pos.X + bestDir.X*moveDist
		proposedY := pos.Y + bestDir.Y*moveDist
		// 边界处理
		proposedX = math.Max(0, math.Min(1000, proposedX))
		proposedY = math.Max(0, math.Min(600, proposedY))
		if !w.CheckCollision(e, proposedX, proposedY) {
			pos.X = proposedX
			pos.Y = proposedY
			mov.VelocityCache = vecScale(bestDir, actualSpeed)
		} else {
			mov.VelocityCache = Vector2{0, 0}
		}
		//打印这段的日志
		log.Printf("Entity %d moved to (%.2f, %.2f) with velocity (%.2f, %.2f)", e, pos.X, pos.Y, mov.VelocityCache.X, mov.VelocityCache.Y)
	}
	w.Unlock()
}

// --------------------
// UpdateAttacks：更新攻击并触发技能（增加 MP 检查和目标快照传递）
// --------------------
func (w *ECSWorld) UpdateAttacks(now time.Time) {
	w.Lock()
	const dtSeconds = 0.1
	for e, atk := range w.attacks {
		atk.MP += atk.MPRegen * dtSeconds
		if atk.MP > 100 {
			atk.MP = 100
		}
		tgtComp, exists := w.targets[e]
		if !exists || tgtComp.Target == 0 {
			continue
		}
		pos, ok := w.positions[e]
		if !ok || pos == nil {
			continue
		}
		tgtPos, ok := w.positions[tgtComp.Target]
		if !ok || tgtPos == nil {
			continue
		}
		dx := tgtPos.X - pos.X
		dy := tgtPos.Y - pos.Y
		dist := math.Sqrt(dx*dx + dy*dy)

		selfCol := w.collisions[e]
		targetCol := w.collisions[tgtComp.Target]
		effectiveRange := atk.AttackRange + selfCol.Radius + targetCol.Radius - 5

		if dist <= effectiveRange && now.Sub(atk.LastAttack) >= atk.AttackSpeed && atk.MP >= atk.MPCost {
			atk.MP -= atk.MPCost
			damage := atk.Attack
			w.ApplyDamage(tgtComp.Target, damage, e)
			atk.LastAttack = now
			log.Printf("Entity %d attacked target %d for %.1f damage", e, tgtComp.Target, damage)
			// 取目标快照
			targetSnapshot := tgtComp.Target
			w.Unlock()
			w.ProcessSkills(e, now, targetSnapshot)
			w.Lock()
		}
	}
	w.Unlock()
	w.CleanupEntities()
}

// --------------------
// GenerateFrame：生成同步数据
// --------------------
func (w *ECSWorld) GenerateFrame(serverTime int64) *pb.GameStateSync {
	w.RLock()
	defer w.RUnlock()
	syncMsg := &pb.GameStateSync{
		SessionId:   1,
		FrameNumber: serverTime,
		ServerTime:  serverTime,
	}
	for e, pos := range w.positions {
		cardInfo := w.cardInfos[e]
		hpComp := w.hps[e]
		var targetID uint64 = 0
		var velX, velY int32 = 0, 0
		if tgt, ok := w.targets[e]; ok && tgt.Target != 0 {
			targetID = uint64(tgt.Target)
		}
		if mov, ok := w.movements[e]; ok {
			velX = int32(mov.VelocityCache.X * 1000)
			velY = int32(mov.VelocityCache.Y * 1000)
		}
		be := &pb.BattleEntity{
			InstanceId:     uint64(e),
			CardId:         cardInfo.CardID,
			EntityType:     pb.EntityType(cardInfo.UnitType),
			Position:       &pb.Position{X: int32(pos.X * 1000), Y: int32(pos.Y * 1000)},
			Velocity:       &pb.Vector2{X: float32(velX), Y: float32(velY)},
			CurrentHp:      int32(hpComp.HP),
			CurrentMp:      0,
			State:          pb.EntityState_MOVING,
			TargetId:       targetID,
			LastAttackTime: 0,
		}
		if growth, ok := w.growth[e]; ok {
			be.Level = int32(growth.Level)
			be.Xp = int32(growth.XP)
		}
		syncMsg.Entities = append(syncMsg.Entities, be)
	}
	return syncMsg
}

// --------------------
// 主循环
// --------------------
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger) // 设为全局默认Logger

	initSkillRegistry()

	world := NewECSWorld()
	world.InitBattlefield()

	// 模拟双方玩家下卡牌（拖放操作）
	card1 := pb.Card{
		Id:          101,
		Name:        "战士卡",
		Attack:      20,
		AttackSpeed: 1000,
		AttackRange: 100,
		Hp:          150,
		Defense:     10,
		MoveSpeed:   80,
		Growth: &pb.Growth{
			Level:       1,
			Attack:      20,
			AttackSpeed: 1000,
			AttackRange: 50,
			Mp:          100,
			MpCost:      10,
			MpRegen:     1,
			Defense:     10,
			Hp:          150,
			HpRegen:     1,
			MoveSpeed:   30,
		},
		MagicSkill: []*pb.MagicSkill{
			{
				Id:              201,
				Name:            "火球术",
				Description:     "召唤从天而降的火球，对目标及周围敌人造成伤害",
				MagicType:       uint32(pb.MagicType_DAMAGE),
				Cooldown:        2000,
				MagicPower:      15,
				IsProjectile:    true,
				ProjectileSpeed: 100,
				RangeShape:      pb.RangeShape_CIRCLE,
				RangeRadius:     50,
			},
			{
				Id:           202,
				Name:         "治疗术",
				Description:  "回复自身少量生命",
				MagicType:    uint32(pb.MagicType_HEAL),
				Cooldown:     3000,
				MagicPower:   10,
				IsProjectile: false,
				TargetType:   int32(pb.MagicTargetType_SELF),
			},
		},
	}
	card2 := pb.Card{
		Id:          102,
		Name:        "弓手卡",
		Attack:      15,
		AttackSpeed: 800,
		AttackRange: 100,
		Hp:          100,
		Defense:     5,
		MoveSpeed:   80,
		Growth: &pb.Growth{
			Level:       1,
			Attack:      15,
			AttackSpeed: 800,
			AttackRange: 60,
			Mp:          100,
			MpCost:      10,
			MpRegen:     1,
			Defense:     5,
			Hp:          100,
			HpRegen:     1,
			MoveSpeed:   40,
		},
		MagicSkill: []*pb.MagicSkill{
			{
				Id:              203,
				Name:            "冰箭",
				Description:     "向敌人射出冰箭，造成伤害并冻结目标",
				MagicType:       uint32(pb.MagicType_DAMAGE),
				Cooldown:        1500,
				MagicPower:      12,
				IsProjectile:    true,
				ProjectileSpeed: 120,
				TargetType:      int32(pb.MagicTargetType_ENEMY),
			},
		},
	}

	// 站位限制：竖屏下，假设玩家1放在上半边（y < 300），玩家2放在下半边（y >= 300）
	action1 := &pb.PlaceCardAction{
		CardId: card1.Id,
		Pos:    &pb.Position{X: 400, Y: 200}, // 玩家1的区域
	}
	action2 := &pb.PlaceCardAction{
		CardId: card2.Id,
		Pos:    &pb.Position{X: 600, Y: 400}, // 玩家2的区域
	}
	world.ProcessPlaceCardAction(action1, card1, 1)
	world.ProcessPlaceCardAction(action2, card2, 2)

	tick := 100 * time.Millisecond
	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	done := make(chan bool)
	frameCounter := 0
	go func() {
		for {
			select {
			case t := <-ticker.C:
				now := t
				world.CleanupExpiredEffects(now)
				world.UpdateTargeting()
				world.UpdateAttacks(now)
				world.UpdateMovement(tick, now)
				frame := world.GenerateFrame(t.UnixMilli())
				frameCounter++
				if frameCounter%10 == 0 {
					log.Printf("[Frame %d] Entity count: %d", frame.ServerTime, len(frame.Entities))
				}
			case <-done:
				return
			}
		}
	}()

	time.Sleep(10 * time.Second)
	done <- true
	log.Println("Battle simulation ended.")
}
