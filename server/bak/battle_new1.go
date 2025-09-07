package new1

import (
	"fmt"
	"log"
	"log/slog"
	"math"
	"os"
	pb "proto"
	"sync/atomic"
	"time"
)

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
// 原子时间封装
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

const (
	UnitCard    = 1
	UnitTower   = 2
	UnitCrystal = 3
)

// --------------------
// 碰撞组件
// --------------------
type CollisionComponent struct {
	ShapeType int
	Radius    float64
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
	Attack      float64
	AttackSpeed time.Duration
	LastAttack  time.Time
	AttackRange float64
	MP          float64
	MPCost      float64
	MPRegen     float64
}

type MovementComponent struct {
	MoveSpeed     float64
	VelocityCache Vector2
}

type TargetComponent struct {
	Target Entity
}

type OwnerComponent struct {
	PlayerID uint64
}

type CardInfoComponent struct {
	CardID   uint64
	UnitType int
}

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
// 事件系统
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
	subscribers map[EventType][]func(interface{})
}

func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[EventType][]func(interface{})),
	}
}

func (bus *EventBus) Subscribe(eventType EventType, handler func(interface{})) {
	bus.subscribers[eventType] = append(bus.subscribers[eventType], handler)
}

func (bus *EventBus) Publish(eventType EventType, event interface{}) {
	if handlers, ok := bus.subscribers[eventType]; ok {
		for _, handler := range handlers {
			handler(event)
		}
	}
}

var eventBus = NewEventBus()

// --------------------
// Actor模型命令
// --------------------
type Command interface{}

type CmdPlaceCard struct {
	Card  pb.Card
	PosX  int
	PosY  int
	Owner uint64
}

type CmdStop struct{}

// --------------------
// ECSWorld（Actor模型改造）
// --------------------
type ECSWorld struct {
	cmdCh          chan Command
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
		cmdCh:        make(chan Command, 100),
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

func (w *ECSWorld) Run() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case cmd := <-w.cmdCh:
			switch c := cmd.(type) {
			case CmdPlaceCard:
				w.handlePlaceCard(c)
			case CmdStop:
				return
			}

		case t := <-ticker.C:
			now := t
			w.cleanupExpiredEffects(now)
			w.updateTargeting()
			w.processAttacks(now)
			w.processMovement(100*time.Millisecond, now)
			w.cleanupEntities()
		}
	}
}

func (w *ECSWorld) Stop() {
	w.cmdCh <- CmdStop{}
}

func (w *ECSWorld) PostCommand(cmd Command) {
	w.cmdCh <- cmd
}

// --------------------
// 实体管理
// --------------------
func (w *ECSWorld) newEntity() Entity {
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

// --------------------
// 战斗初始化
// --------------------
func (w *ECSWorld) InitBattlefield() {
	// 初始化代码与原来相同...
}

// --------------------
// 卡牌放置处理
// --------------------
func (w *ECSWorld) handlePlaceCard(cmd CmdPlaceCard) {
	posX := float64(cmd.PosX)
	posY := float64(cmd.PosY)

	// 区域检查
	if cmd.Owner == 1 && posY >= 300 {
		return
	}
	if cmd.Owner == 2 && posY < 300 {
		return
	}

	e := w.newEntity()
	w.positions[e] = &PositionComponent{X: posX, Y: posY}
	// 其他组件初始化与原来类似...
	eventBus.Publish(EntitySpawn, e)
}

// --------------------
// 碰撞检测
// --------------------
func (w *ECSWorld) checkCollision(e Entity, newX, newY float64) bool {
	// 实现与原来相同（去掉锁）...
}

// --------------------
// 效果系统
// --------------------
func (w *ECSWorld) applyEffect(target Entity, effectType int, value float64, duration time.Duration, now time.Time) {
	// 实现与原来相同（去掉锁）...
}

// --------------------
// 技能处理（直接处理不再需要注册）
// --------------------
func (w *ECSWorld) processSkills(e Entity, now time.Time, target Entity) {
	if sComp := w.skills[e]; sComp != nil {
		for _, s := range sComp.Skills {
			if now.Sub(s.LastUsed.Load()) < time.Duration(s.Skill.Cooldown)*time.Millisecond {
				continue
			}

			if !w.canCastSkill(e, s, target) {
				continue
			}

			switch s.Skill.Name {
			case "火球术":
				w.handleFireball(e, target, s.Skill, now)
			case "冰箭":
				w.handleIceArrow(e, target, s.Skill, now)
			case "治疗术":
				w.handleHeal(e, s.Skill, now)
			}

			s.LastUsed.Store(now)
			eventBus.Publish(SkillCast, fmt.Sprintf("Entity %d cast %s", e, s.Skill.Name))
		}
	}
}

// 具体技能实现
func (w *ECSWorld) handleFireball(caster, target Entity, skill pb.MagicSkill, now time.Time) {
	// 火球术实现...
}

func (w *ECSWorld) handleIceArrow(caster, target Entity, skill pb.MagicSkill, now time.Time) {
	// 冰箭实现...
}

func (w *ECSWorld) handleHeal(caster Entity, skill pb.MagicSkill, now time.Time) {
	// 治疗术实现...
}

// --------------------
// 其他系统（攻击、移动、目标选择等）
// --------------------
func (w *ECSWorld) updateTargeting() {
	// 实现与原来相同（去掉锁）...
}

func (w *ECSWorld) processMovement(dt time.Duration, now time.Time) {
	// 实现与原来相同（去掉锁）...
}

func (w *ECSWorld) processAttacks(now time.Time) {
	// 实现与原来相同（去掉锁）...
}

// --------------------
// 主函数
// --------------------
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	world := NewECSWorld()
	world.InitBattlefield()

	// 启动Actor主循环
	go world.Run()

	// 发送放置卡牌命令
	card1 := pb.Card{ /* 卡牌定义 */ }
	world.PostCommand(CmdPlaceCard{
		Card:  card1,
		PosX:  400,
		PosY:  200,
		Owner: 1,
	})

	// 运行一段时间后停止
	time.Sleep(30 * time.Second)
	world.Stop()
	log.Println("Simulation stopped")
}
