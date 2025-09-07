package

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"time"

	pb "proto"
)

// --------------------
// 异步日志通道
// --------------------
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
// 向量与基础函数
// --------------------
type Vector2 struct{ X, Y float64 }

func vecLength(v Vector2) float64 {
	return math.Hypot(v.X, v.Y)
}
func vecNormalize(v Vector2) Vector2 {
	l := vecLength(v)
	if l == 0 {
		return Vector2{}
	}
	return Vector2{v.X / l, v.Y / l}
}
func vecAdd(a, b Vector2) Vector2           { return Vector2{a.X + b.X, a.Y + b.Y} }
func vecSub(a, b Vector2) Vector2           { return Vector2{a.X - b.X, a.Y - b.Y} }
func vecScale(v Vector2, s float64) Vector2 { return Vector2{v.X * s, v.Y * s} }
func rotateVector(v Vector2, deg float64) Vector2 {
	rad := deg * math.Pi / 180
	return Vector2{
		X: v.X*math.Cos(rad) - v.Y*math.Sin(rad),
		Y: v.X*math.Sin(rad) + v.Y*math.Cos(rad),
	}
}

// --------------------
// 原子时间封装
// --------------------
type atomicTime struct{ v atomic.Int64 }

func (a *atomicTime) Store(t time.Time) { a.v.Store(t.UnixNano()) }
func (a *atomicTime) Load() time.Time   { return time.Unix(0, a.v.Load()) }

// --------------------
// 枚举与常量
// --------------------
type Entity uint64

const (
	UnitCard    = 1
	UnitTower   = 2
	UnitCrystal = 3
)
const CollisionCircle = 1

// --------------------
// 组件结构定义
// --------------------
type PositionComponent struct{ X, Y float64 }
type HPComponent struct{ HP float64 }
type AttackComponent struct {
	Attack              float64
	AttackSpeed         time.Duration
	LastAttack          time.Time
	AttackRange         float64
	MP, MPCost, MPRegen float64
}
type MovementComponent struct {
	MoveSpeed     float64
	VelocityCache Vector2
}
type TargetComponent struct{ Target Entity }
type OwnerComponent struct{ PlayerID uint64 }
type CardInfoComponent struct {
	CardID   uint64
	UnitType int
}
type Skill struct {
	Skill    pb.MagicSkill
	LastUsed atomicTime
}
type SkillComponent struct{ Skills map[uint64]*Skill }
type GrowthComponent struct{ Level, XP int }
type EffectComponent struct {
	EffectType int
	Value      float64
	Duration   time.Duration
	StartTime  time.Time
	Priority   int
}
type CollisionComponent struct {
	ShapeType int
	Radius    float64
}

// --------------------
// 事件总线
// --------------------
type EventType int

const (
	EntitySpawn EventType = iota
	EntityDeath
	SkillCast
)

type DeathEvent struct{ Entity, Killer Entity }

type EventBus struct {
	sync.RWMutex
	subscribers map[EventType][]func(interface{})
}

func NewEventBus() *EventBus {
	return &EventBus{subscribers: make(map[EventType][]func(interface{}))}
}
func (bus *EventBus) Subscribe(t EventType, h func(interface{})) {
	bus.Lock()
	defer bus.Unlock()
	bus.subscribers[t] = append(bus.subscribers[t], h)
}
func (bus *EventBus) Publish(t EventType, e interface{}) {
	bus.RLock()
	handlers := bus.subscribers[t]
	bus.RUnlock()
	for _, h := range handlers {
		go h(e)
	}
}

var eventBus = NewEventBus()

// --------------------
// 技能配置与注册
// --------------------
type SkillConfig struct {
	SkillID uint64                 `json:"skill_id"`
	Name    string                 `json:"name"`
	Handler string                 `json:"handler"`
	Params  map[string]interface{} `json:"params"`
}

var skillConfigs map[uint64]SkillConfig

func loadSkillConfigs() {
	var arr []SkillConfig
	_ = json.Unmarshal([]byte(`[
        {"skill_id":201,"name":"火球术","handler":"FireballHandler","params":{"radius":5.0,"damage_multiplier":1.2}}
    ]`), &arr)
	skillConfigs = make(map[uint64]SkillConfig, len(arr))
	for _, c := range arr {
		skillConfigs[c.SkillID] = c
	}
}

type SkillHandler interface {
	Execute(w *ECSWorld, caster, target Entity, ms pb.MagicSkill, now time.Time)
}

var skillRegistry = make(map[string]SkillHandler)

func RegisterSkill(name string, h SkillHandler) { skillRegistry[name] = h }

// 火球术
type FireballHandler struct{}

func (h *FireballHandler) Execute(w *ECSWorld, caster, target Entity, ms pb.MagicSkill, now time.Time) {
	// 读取配置
	radius := float64(ms.RangeRadius)
	dmg := float64(ms.MagicPower)
	if cfg, ok := skillConfigs[ms.Id]; ok {
		if r, ok2 := cfg.Params["radius"].(float64); ok2 {
			radius = r
		}
		if m, ok2 := cfg.Params["damage_multiplier"].(float64); ok2 {
			dmg *= m
		}
	}

	// 读取目标位置及世界快照
	w.posLock.RLock()
	tpos, ok := w.positions[target]
	if !ok {
		w.posLock.RUnlock()
		return
	}
	// 复制 positions
	snap := make([]struct {
		e   Entity
		pos Vector2
	}, 0, len(w.positions))
	for e, p := range w.positions {
		snap = append(snap, struct {
			e   Entity
			pos Vector2
		}{e, Vector2{p.X, p.Y}})
	}
	w.posLock.RUnlock()

	for _, item := range snap {
		if math.Hypot(item.pos.X-tpos.X, item.pos.Y-tpos.Y) <= radius {
			w.ApplyDamage(item.e, dmg, caster)
			asyncLog("Fireball: %d->%d damage %.1f", caster, item.e, dmg)
		}
	}
}

// 冰箭
type IceArrowHandler struct{}

func (h *IceArrowHandler) Execute(w *ECSWorld, caster, target Entity, ms pb.MagicSkill, now time.Time) {
	dmg := float64(ms.MagicPower)
	w.hpLock.Lock()
	if hp, ok := w.hps[target]; ok {
		hp.HP -= dmg
	}
	w.hpLock.Unlock()
	w.ApplyEffect(target, EFFECT_FREEZE, 1.0, 1500*time.Millisecond, now)
	asyncLog("IceArrow: %d->%d damage %.1f", caster, target, dmg)
}

// 治疗术
type HealHandler struct{}

func (h *HealHandler) Execute(w *ECSWorld, caster, target Entity, ms pb.MagicSkill, now time.Time) {
	w.hpLock.Lock()
	if hp, ok := w.hps[caster]; ok {
		hp.HP += 20
	}
	w.hpLock.Unlock()
	asyncLog("Heal: %d healed self", caster)
}

func initSkills() {
	loadSkillConfigs()
	RegisterSkill("火球术", &FireballHandler{})
	RegisterSkill("冰箭", &IceArrowHandler{})
	RegisterSkill("治疗术", &HealHandler{})
}

// --------------------
// ECSWorld（Actor 模型）
// --------------------
type ECSCommand interface{}
type CmdPlaceCard struct {
	Card       pb.Card
	PosX, PosY int
	Owner      uint64
}
type CmdStop struct{}

type ECSWorld struct {
	// 命令通道
	cmdCh chan ECSCommand
	// 实体ID原子
	nextID atomic.Uint64

	// pending removal
	removalLock    sync.Mutex
	pendingRemoval []Entity

	// 各组件及锁
	posLock    sync.RWMutex
	positions  map[Entity]*PositionComponent
	hpLock     sync.RWMutex
	hps        map[Entity]*HPComponent
	atkLock    sync.RWMutex
	attacks    map[Entity]*AttackComponent
	movLock    sync.RWMutex
	movements  map[Entity]*MovementComponent
	tgtLock    sync.RWMutex
	targets    map[Entity]*TargetComponent
	ownLock    sync.RWMutex
	owners     map[Entity]*OwnerComponent
	infoLock   sync.RWMutex
	cardInfos  map[Entity]*CardInfoComponent
	sklLock    sync.RWMutex
	skills     map[Entity]*SkillComponent
	grwLock    sync.RWMutex
	growth     map[Entity]*GrowthComponent
	effLock    sync.RWMutex
	effects    map[Entity][]*EffectComponent
	colLock    sync.RWMutex
	collisions map[Entity]*CollisionComponent
}

func NewECSWorld() *ECSWorld {
	w := &ECSWorld{
		cmdCh:      make(chan ECSCommand, 100),
		positions:  make(map[Entity]*PositionComponent),
		hps:        make(map[Entity]*HPComponent),
		attacks:    make(map[Entity]*AttackComponent),
		movements:  make(map[Entity]*MovementComponent),
		targets:    make(map[Entity]*TargetComponent),
		owners:     make(map[Entity]*OwnerComponent),
		cardInfos:  make(map[Entity]*CardInfoComponent),
		skills:     make(map[Entity]*SkillComponent),
		growth:     make(map[Entity]*GrowthComponent),
		effects:    make(map[Entity][]*EffectComponent),
		collisions: make(map[Entity]*CollisionComponent),
	}
	return w
}

// 启动 Actor 循环
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
			w.cleanupEffects(now)
			w.acquireTargets()
			w.processAttacks(now)
			w.processMovement(now)
			w.cleanupEntities()
		}
	}
}

func (w *ECSWorld) Stop() {
	w.cmdCh <- CmdStop{}
}

func (w *ECSWorld) NextEntity() Entity {
	return Entity(w.nextID.Add(1))
}

// --------------------
// 处理放牌
// --------------------
func (w *ECSWorld) handlePlaceCard(c CmdPlaceCard) {
	e := w.NextEntity()
	// position
	w.posLock.Lock()
	w.positions[e] = &PositionComponent{X: float64(c.PosX), Y: float64(c.PosY)}
	w.posLock.Unlock()
	// hp
	w.hpLock.Lock()
	w.hps[e] = &HPComponent{HP: float64(c.Card.Hp)}
	w.hpLock.Unlock()
	// owner
	w.ownLock.Lock()
	w.owners[e] = &OwnerComponent{PlayerID: c.Owner}
	w.ownLock.Unlock()
	// cardInfo
	w.infoLock.Lock()
	w.cardInfos[e] = &CardInfoComponent{CardID: c.Card.Id, UnitType: UnitCard}
	w.infoLock.Unlock()
	// attack
	w.atkLock.Lock()
	w.attacks[e] = &AttackComponent{
		Attack:      float64(c.Card.Attack),
		AttackSpeed: time.Duration(c.Card.AttackSpeed) * time.Millisecond,
		LastAttack:  time.Now(),
		AttackRange: float64(c.Card.AttackRange),
		MP:          100, MPCost: 10, MPRegen: 1,
	}
	w.atkLock.Unlock()
	// movement
	w.movLock.Lock()
	w.movements[e] = &MovementComponent{MoveSpeed: float64(c.Card.MoveSpeed)}
	w.movLock.Unlock()
	// target
	w.tgtLock.Lock()
	w.targets[e] = &TargetComponent{Target: 0}
	w.tgtLock.Unlock()
	// skill
	w.sklLock.Lock()
	sc := &SkillComponent{Skills: make(map[uint64]*Skill)}
	for _, ms := range c.Card.MagicSkill {
		s := &Skill{Skill: *ms}
		s.LastUsed.Store(time.Now().Add(-time.Duration(ms.Cooldown) * time.Millisecond))
		sc.Skills[ms.Id] = s
	}
	w.skills[e] = sc
	w.sklLock.Unlock()
	// growth
	w.grwLock.Lock()
	w.growth[e] = &GrowthComponent{Level: int(c.Card.Growth.Level), XP: 0}
	w.grwLock.Unlock()
	// effects
	w.effLock.Lock()
	w.effects[e] = nil
	w.effLock.Unlock()
	// collision
	w.colLock.Lock()
	w.collisions[e] = &CollisionComponent{ShapeType: CollisionCircle, Radius: 20}
	w.colLock.Unlock()

	asyncLog("[Server] PlaceCard: card %d entity %d at (%d,%d) for player %d",
		c.Card.Id, e, c.PosX, c.PosY, c.Owner)
	eventBus.Publish(EntitySpawn, e)
}

// --------------------
// 系统实现
// --------------------
func (w *ECSWorld) cleanupEffects(now time.Time) {
	w.effLock.Lock()
	defer w.effLock.Unlock()
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

func (w *ECSWorld) acquireTargets() {
	w.atkLock.RLock()
	defer w.atkLock.RUnlock()
	w.tgtLock.Lock()
	defer w.tgtLock.Unlock()
	for e := range w.attacks {
		// reset or keep
		if t := w.targets[e].Target; t != 0 {
			if hp, ok := w.hps[t]; !ok || hp.HP <= 0 {
				w.targets[e].Target = 0
			} else {
				continue
			}
		}
		// find nearest
		w.posLock.RLock()
		pos := w.positions[e]
		w.posLock.RUnlock()
		minD := math.MaxFloat64
		var best Entity
		w.hpLock.RLock()
		for o, h := range w.hps {
			if o == e || h.HP <= 0 {
				continue
			}
			w.ownLock.RLock()
			if w.owners[o].PlayerID == w.owners[e].PlayerID {
				w.ownLock.RUnlock()
				continue
			}
			w.ownLock.RUnlock()
			w.posLock.RLock()
			opos := w.positions[o]
			w.posLock.RUnlock()
			d := math.Hypot(opos.X-pos.X, opos.Y-pos.Y)
			if d < minD {
				minD = d
				best = o
			}
		}
		w.hpLock.RUnlock()
		if best != 0 {
			w.targets[e].Target = best
			asyncLog("Entity %d targets %d", e, best)
		}
	}
}

func (w *ECSWorld) processAttacks(now time.Time) {
	w.atkLock.Lock()
	defer w.atkLock.Unlock()
	for e, atk := range w.attacks {
		atk.MP += atk.MPRegen * 0.1
		if atk.MP > 100 {
			atk.MP = 100
		}
		tgt := w.targets[e].Target
		if tgt == 0 {
			continue
		}
		w.posLock.RLock()
		p := w.positions[e]
		tp := w.positions[tgt]
		w.posLock.RUnlock()
		d := math.Hypot(tp.X-p.X, tp.Y-p.Y)
		w.colLock.RLock()
		sr := atk.AttackRange + w.collisions[e].Radius + w.collisions[tgt].Radius - 5
		w.colLock.RUnlock()
		if d <= sr && now.Sub(atk.LastAttack) >= atk.AttackSpeed && atk.MP >= atk.MPCost {
			atk.MP -= atk.MPCost
			atk.LastAttack = now
			w.ApplyDamage(tgt, atk.Attack, e)
			asyncLog("Entity %d attacked %d for %.1f", e, tgt, atk.Attack)
			// 技能触发
			w.processSkills(e, now, tgt)
		}
	}
}

func (w *ECSWorld) processSkills(e Entity, now time.Time, target Entity) {
	w.sklLock.RLock()
	var list []*Skill
	for _, s := range w.skills[e].Skills {
		list = append(list, s)
	}
	w.sklLock.RUnlock()
	for _, s := range list {
		if now.Sub(s.LastUsed.Load()) < time.Duration(s.Skill.Cooldown)*time.Millisecond {
			continue
		}
		// 目标合法
		if handler, ok := skillRegistry[s.Skill.Name]; ok {
			handler.Execute(w, e, target, s.Skill, now)
			s.LastUsed.Store(now)
			eventBus.Publish(SkillCast, fmt.Sprintf("Entity %d cast %s", e, s.Skill.Name))
		}
	}
}

func (w *ECSWorld) processMovement(now time.Time) {
	w.movLock.Lock()
	defer w.movLock.Unlock()
	for e, mov := range w.movements {
		tgt := w.targets[e].Target
		if tgt == 0 {
			mov.VelocityCache = Vector2{}
			continue
		}
		w.posLock.RLock()
		p := w.positions[e]
		tp := w.positions[tgt]
		w.posLock.RUnlock()
		dir := vecNormalize(Vector2{tp.X - p.X, tp.Y - p.Y})
		speed := mov.MoveSpeed // 可加入减速效果
		mv := vecScale(dir, speed*0.1)
		newP := Vector2{p.X + mv.X, p.Y + mv.Y}
		w.posLock.Lock()
		w.positions[e] = &PositionComponent{newP.X, newP.Y}
		w.posLock.Unlock()
		mov.VelocityCache = mv
		asyncLog("Entity %d moved to (%.2f,%.2f)", e, newP.X, newP.Y)
	}
}

func (w *ECSWorld) cleanupEntities() {
	w.removalLock.Lock()
	rem := w.pendingRemoval
	w.pendingRemoval = nil
	w.removalLock.Unlock()
	for _, e := range rem {
		w.posLock.Lock()
		delete(w.positions, e)
		w.posLock.Unlock()
		w.hpLock.Lock()
		delete(w.hps, e)
		w.hpLock.Unlock()
		w.atkLock.Lock()
		delete(w.attacks, e)
		w.atkLock.Unlock()
		w.movLock.Lock()
		delete(w.movements, e)
		w.movLock.Unlock()
		w.tgtLock.Lock()
		delete(w.targets, e)
		w.tgtLock.Unlock()
		w.ownLock.Lock()
		delete(w.owners, e)
		w.ownLock.Unlock()
		w.infoLock.Lock()
		delete(w.cardInfos, e)
		w.infoLock.Unlock()
		w.sklLock.Lock()
		delete(w.skills, e)
		w.sklLock.Unlock()
		w.grwLock.Lock()
		delete(w.growth, e)
		w.grwLock.Unlock()
		w.effLock.Lock()
		delete(w.effects, e)
		w.effLock.Unlock()
		w.colLock.Lock()
		delete(w.collisions, e)
		w.colLock.Unlock()
		asyncLog("Entity %d removed", e)
	}
}

// ApplyDamage 与 ApplyEffect
func (w *ECSWorld) ApplyDamage(target Entity, dmg float64, attacker Entity) {
	w.hpLock.Lock()
	if hp, ok := w.hps[target]; ok {
		hp.HP -= dmg
		if hp.HP <= 0 {
			w.removalLock.Lock()
			w.pendingRemoval = append(w.pendingRemoval, target)
			w.removalLock.Unlock()
			eventBus.Publish(EntityDeath, DeathEvent{Entity: target, Killer: attacker})
		}
	}
	w.hpLock.Unlock()
}

func (w *ECSWorld) ApplyEffect(target Entity, et int, val float64, dur time.Duration, now time.Time) {
	w.effLock.Lock()
	w.effects[target] = append(w.effects[target], &EffectComponent{
		EffectType: et, Value: val, Duration: dur, StartTime: now, Priority: 1,
	})
	w.effLock.Unlock()
}

// --------------------
// 主函数
// --------------------
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(logger)

	initSkills()
	world := NewECSWorld()

	// 订阅事件示例
	eventBus.Subscribe(EntityDeath, func(e interface{}) {
		d := e.(DeathEvent)
		asyncLog("Event: %d killed by %d", d.Entity, d.Killer)
	})

	go world.Run()

	// 放两张卡作演示
	world.cmdCh <- CmdPlaceCard{Card: pb.Card{Id: 101, Hp: 150, Attack: 20, AttackSpeed: 1000, AttackRange: 100, MoveSpeed: 80,
		Growth: &pb.Growth{Level: 1}, MagicSkill: []*pb.MagicSkill{{Id: 201, Name: "火球术", Cooldown: 2000, MagicPower: 15, RangeRadius: 50}},
	}, PosX: 400, PosY: 200, Owner: 1}

	world.cmdCh <- CmdPlaceCard{Card: pb.Card{Id: 102, Hp: 100, Attack: 15, AttackSpeed: 800, AttackRange: 100, MoveSpeed: 80,
		Growth: &pb.Growth{Level: 1}, MagicSkill: []*pb.MagicSkill{{Id: 203, Name: "冰箭", Cooldown: 1500, MagicPower: 12}},
	}, PosX: 600, PosY: 400, Owner: 2}

	// 运行 5 秒后停止
	time.Sleep(5 * time.Second)
	world.Stop()
	time.Sleep(100 * time.Millisecond)
	log.Println("Simulation ended.")
}
