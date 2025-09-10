package main

import (
	cfg "cfg"
	"encoding/json"
	"errors"
	"fmt"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"math/rand"
	"os"
	pb "proto"
	"strings"
	"time"
)

// 1. 定义玩家上下文接口(新增)
type PlayerDrawCardContext interface {
	GetVIPLevel() int
	GetDrawCount() int32
	// 可扩展其他需要的方法
	GetPlayerID() uint64
	SetDrawCount(count int32)
	SaveDrawCardResults(cards []*pb.Card)
}

// 2. 定义概率调整接口(新增)
type ProbabilityAdjuster interface {
	AdjustRarityWeights(ctx PlayerDrawCardContext, originalWeights map[int32]float64) map[int32]float64
}

// 3. 定义卡牌生成拦截器接口(新增)
type CardGenerateInterceptor interface {
	BeforeGenerate(s *CardService, ctx PlayerDrawCardContext, baseCard *pb.Card) *pb.Card
	AfterGenerate(ctx PlayerDrawCardContext, generatedCard *pb.Card) *pb.Card
}

// 玩家数据仓库接口(新增)
type PlayerRepository interface {
	GetPlayerContext(uid uint64) (PlayerDrawCardContext, error)
}

func JsonLoader(filename string) ([]map[string]interface{}, error) {
	file, err := os.Open("../cfg/" + filename + ".json")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var data []map[string]interface{}
	if err := json.NewDecoder(file).Decode(&data); err != nil {
		return nil, err
	}
	return data, nil
}

var CardSvc *CardService

func init() {
	CardConfig, err := cfg.NewTables(JsonLoader)
	if err != nil {
		slog.Error("Failed to load card config", "error", err)
		panic(err)
	}

	// 使用 tables.TbDrawCard 获取配置数据
	// 例如，获取 rarity_level 为 1 的配置
	//drawCard := CardConfig.TbDrawCard.Get(1)
	//if drawCard != nil {
	//	fmt.Printf("RarityLevel: %d\n", drawCard.RarityLevel)
	//	fmt.Printf("Rarity Prefix: %s\n", drawCard.Rarity.Prefix)
	//	fmt.Printf("BaseStats MaxLevel: %d\n", drawCard.BaseStats.MaxLevel)
	//	fmt.Printf("Growth AttackPerLevel: %d\n", drawCard.Growth.AttackPerLevel)
	//}

	CardSvc = NewCardService(CardConfig, &DefaultPlayerRepo{}, []ProbabilityAdjuster{&VIPProbabilityAdjuster{}}, []CardGenerateInterceptor{&GuaranteeInterceptor{}})
}

type CardService struct {
	tables        *cfg.Tables
	probAdjusters []ProbabilityAdjuster     // 概率调整链
	interceptors  []CardGenerateInterceptor // 生成拦截器链
	playerRepo    PlayerRepository          // 玩家数据仓库接口
}

func NewCardService(tables *cfg.Tables, playerRepo PlayerRepository, adjusters []ProbabilityAdjuster, interceptors []CardGenerateInterceptor) *CardService {
	rand.Seed(time.Now().UnixNano())
	//return &CardService{tables: tables}
	return &CardService{
		tables:        tables,
		playerRepo:    playerRepo,
		probAdjusters: adjusters,
		interceptors:  interceptors,
	}
}

// 核心抽卡方法
func (s *CardService) DrawCards(uid uint64, count int32) *pb.DrawCardResponse {

	// 获取玩家上下文
	ctx, err := s.playerRepo.GetPlayerContext(uid)
	if err != nil {
		return &pb.DrawCardResponse{
			Ret: pb.ErrorCode_INVALID_CARD,
		}
	}

	resp := &pb.DrawCardResponse{
		Ret:   pb.ErrorCode_OK,
		Cards: make([]*pb.Card, 0, count),
	}

	for i := int32(0); i < count; i++ {
		card, err := s.generateCardWithContext(ctx)
		if err == nil {
			resp.Cards = append(resp.Cards, card)
		}
	}

	// 记录抽卡次数等后置逻辑
	s.afterDraw(ctx, resp.Cards)
	return resp
}

// 7. 带上下文的卡牌生成方法(新增)
func (s *CardService) generateCardWithContext(ctx PlayerDrawCardContext) (*pb.Card, error) {
	// 调整后的稀有度配置
	adjustedRarity := s.selectRarityWithContext(ctx)

	// 基础卡牌生成
	baseCard := s.buildCardBase(adjustedRarity)

	// 前置拦截处理, 用作保底等逻辑
	for _, interceptor := range s.interceptors {
		baseCard = interceptor.BeforeGenerate(s, ctx, baseCard)
	}

	// 应用属性
	s.randomizeAttributes(baseCard, adjustedRarity.BaseStats)

	// 后置拦截处理
	for _, interceptor := range s.interceptors {
		baseCard = interceptor.AfterGenerate(ctx, baseCard)
	}

	return baseCard, nil
}

func (s *CardService) selectRarityWithContext(ctx PlayerDrawCardContext) *cfg.CfgDrawCard {
	// 获取原始权重
	originalWeights := make(map[int32]float64)
	for _, r := range s.getAllRarities() {
		originalWeights[r.RarityLevel] = float64(r.Rarity.Weight)
	}

	// 应用调整链
	adjustedWeights := originalWeights
	for _, adjuster := range s.probAdjusters {
		adjustedWeights = adjuster.AdjustRarityWeights(ctx, adjustedWeights)
	}

	// 根据调整后的权重选择
	total := 0.0
	for _, w := range adjustedWeights {
		total += w
	}

	randomValue := rand.Float64() * total
	for rarityLevel, weight := range adjustedWeights {
		if randomValue <= weight {
			return s.tables.TbDrawCard.Get(rarityLevel)
		}
		randomValue -= weight
	}
	return s.getAllRarities()[0]
}

// 9. 后置处理逻辑(新增)
func (s *CardService) afterDraw(ctx PlayerDrawCardContext, cards []*pb.Card) {
	// 可扩展记录抽卡次数、更新保底计数等逻辑
	ctx.SaveDrawCardResults(cards)
}

// 10. 默认玩家仓库实现(示例)
type DefaultPlayerRepo struct{}

func (r *DefaultPlayerRepo) GetPlayerContext(uid uint64) (PlayerDrawCardContext, error) {
	// 默认实现，后续可替换为真实数据源
	player, ok := GlobalManager.GetPlayerByUin(uid)
	if !ok {
		return nil, errors.New("player not found")
	}

	//player 转化为 PlayerContext
	return player, nil
}

type VIPProbabilityAdjuster struct{}

func (a *VIPProbabilityAdjuster) AdjustRarityWeights(ctx PlayerDrawCardContext, weights map[int32]float64) map[int32]float64 {
	adjusted := make(map[int32]float64)
	for lv, w := range weights {
		// VIP每级提升SSR概率1%
		if lv == 3 { // 假设3是SSR的rarity_level
			adjusted[lv] = w * (1 + 0.01*float64(ctx.GetVIPLevel()))
		} else {
			adjusted[lv] = w
		}
	}
	return adjusted
}

// 保底拦截器
type GuaranteeInterceptor struct {
}

func (i *GuaranteeInterceptor) BeforeGenerate(s *CardService, ctx PlayerDrawCardContext, baseCard *pb.Card) *pb.Card {
	count := ctx.GetDrawCount()

	// 每50抽必出SSR
	if count >= 49 {
		if ssrCfg := s.tables.TbDrawCard.Get(3); ssrCfg != nil {
			baseCard = s.buildCardBase(ssrCfg)
		}
	}
	return baseCard
}

func (i *GuaranteeInterceptor) AfterGenerate(ctx PlayerDrawCardContext, card *pb.Card) *pb.Card {

	if card.Rarity < 3 { // 非SSR时计数+1
		ctx.SetDrawCount(ctx.GetDrawCount() + 1)
	} else {
		ctx.SetDrawCount(0) // 重置计数
	}
	return card
}

// 构建卡牌基础数据
func (s *CardService) buildCardBase(rarity *cfg.CfgDrawCard) *pb.Card {
	// 生成卡牌编号
	number := rand.Int31n(rarity.Rarity.MaxNumber-rarity.Rarity.MinNumber+1) + rarity.Rarity.MinNumber

	return &pb.Card{
		Id:          generateCardID(),
		Name:        formatCardName(rarity.Rarity.Prefix, number),
		Description: formatDescription(rarity.Rarity.Prefix),
		Image:       formatImagePath(rarity.Rarity.Prefix, number),
	}
}

// 属性随机化处理
func (s *CardService) randomizeAttributes(card *pb.Card, stats *cfg.CfgBaseStats) {
	// 攻击力随机
	//if stats.Attack != nil {
	//	card.Attack = randomizeValue(stats.Attack.Base, stats.Attack.Variance)
	//}
	//
	//// 防御力随机
	//if stats.Defense != nil {
	//	card.Defense = randomizeValue(stats.Defense.Base, stats.Defense.Variance)
	//}
	//
	//// 固定属性
	//card.AttackSpeed = stats.AttackSpeed
	//card.AttackRange = stats.AttackRange
	//card.Mp = stats.Mp
	//card.MpCost = stats.MpCost
	//card.MpRegen = stats.MpRegen
	//card.Hp = stats.Hp
	//card.HpRegen = stats.HpRegen
	//card.MoveSpeed = stats.MoveSpeed
}

// 辅助方法
func (s *CardService) calculateTotalWeight() float64 {
	var total float64
	for _, rarity := range s.getAllRarities() {
		total += float64(rarity.Rarity.Weight)
	}
	return total
}

func (s *CardService) getAllRarities() []*cfg.CfgDrawCard {
	var rarities []*cfg.CfgDrawCard
	for _, v := range s.tables.TbDrawCard.GetDataMap() {
		rarities = append(rarities, v)
	}
	return rarities
}

func formatCardName(prefix string, number int32) string {
	//return strings.ToUpper(prefix) + "_" + fmt.Sprintf("%04d", number)
	return prefix + "_" + fmt.Sprintf("%04d", number)
}

func formatImagePath(prefix string, number int32) string {
	return fmt.Sprintf("Art/Cards/%s/%04d", prefix, number)
}

func formatDescription(prefix string) string {
	return fmt.Sprintf("Rarity: %s", strings.ToUpper(prefix))
}

func randomizeValue(base int32, variance float32) int32 {
	min := float64(base) * (1 - float64(variance))
	max := float64(base) * (1 + float64(variance))
	return int32(min + rand.Float64()*(max-min))
}

//func convertGrowth(g *cfg.CfgGrowth) *pb.Growth {
//	return &pb.Growth{
//		Attack:  g.AttackPerLevel,
//		Hp:      g.HpPerLevel,
//		Defense: g.DefensePerLevel,
//		Mp:      g.MpPerLevel,
//	}
//}

var idSeq uint64 = 99999

func generateCardID() uint64 {

	// 使用 SetNX 设置初始值（如果键不存在）
	set, err := GlobalRedis.SetNX("global:card_inst_id", "99999")
	if err != nil {
		slog.Error("Failed to initialize card ID in Redis", "error", err)
		return 0
	}

	if set {
		slog.Info("Initialized card ID counter to 99999")
	}

	// 使用 INCR 获取递增的唯一值
	cardID, err := GlobalRedis.Incr("global:card_inst_id")
	if err != nil {
		slog.Error("Failed to generate card ID using Redis", "error", err)
		return 0
	}

	return uint64(cardID)

}

// 错误定义
var (
	ErrRarityConfigNotFound = errors.New("rarity config not found")
)

func (p *Player) HandleDrawCardRequest(msg *pb.Message) {
	var req pb.DrawCardRequest
	if err := proto.Unmarshal(msg.GetData(), &req); err != nil {
		slog.Error("Failed to parse draw card Request", "error", err)
		return
	}

	// 2. 执行抽卡逻辑
	resp := CardSvc.DrawCards(req.Uid, req.Count)

	slog.Info("Draw card request", "uid", req.Uid, "count", req.Count, "response", resp)

	p.SendResponse(msg, mustMarshal(resp))

}
