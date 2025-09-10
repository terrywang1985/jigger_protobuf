// desktop_pet_handler.go
package main

////import (
////	"encoding/json"
////	"fmt"
////	"github.com/garyburd/redigo/redis"
////	"google.golang.org/protobuf/proto"
////	"log/slog"
////	pb "proto"
////)
////
////// 初始化时注册消息处理器
////func init() {
////	MsgHandler.RegisterHandler(pb.DesktopPetMessageId_DESKTOP_PET_AUTH_REQUEST, handleDesktopPetAuthRequest)
////	MsgHandler.RegisterHandler(pb.DesktopPetMessageId_GET_MARKET_ITEMS_REQUEST, handleGetMarketItemsRequest)
////	MsgHandler.RegisterHandler(pb.DesktopPetMessageId_GET_BACKPACK_ITEMS_REQUEST, handleGetBackpackItemsRequest)
////	MsgHandler.RegisterHandler(pb.DesktopPetMessageId_PURCHASE_REQUEST, handlePurchaseRequest)
////	MsgHandler.RegisterHandler(pb.DesktopPetMessageId_EQUIP_REQUEST, handleEquipRequest)
////	MsgHandler.RegisterHandler(pb.DesktopPetMessageId_GET_SKIN_DATA_REQUEST, handleGetSkinDataRequest)
////}
////
////// 处理桌面宠物认证请求
////func handleDesktopPetAuthRequest(p *Player, msg *pb.Message) {
////	p.HandleDesktopPetAuthRequest(msg)
////}
//
//// 处理获取商城物品请求
//func handleGetMarketItemsRequest(p *Player, msg *pb.Message) {
//	// 从Redis获取商城物品列表
//	marketItems, err := getMarketItemsFromRedis()
//	if err != nil {
//		slog.Error("Failed to get market items", "error", err)
//		p.SendResponse(msg, mustMarshal(&pb.MarketItemsResponse{
//			Ret: pb.ErrorCode_SERVER_ERROR,
//		}))
//		return
//	}
//
//	// 标记用户已拥有的物品
//	for i := range marketItems {
//		if p.hasItem(marketItems[i].Id) {
//			marketItems[i].Owned = true
//			if p.isItemEquipped(marketItems[i].Id) {
//				marketItems[i].Equipped = true
//			}
//		}
//	}
//
//	// 发送响应
//	p.SendResponse(msg, mustMarshal(&pb.MarketItemsResponse{
//		Ret:   pb.ErrorCode_OK,
//		Items: marketItems,
//	}))
//}
//
//// 处理获取背包物品请求
//func handleGetBackpackItemsRequest(p *Player, msg *pb.Message) {
//	// 从Redis获取用户背包物品
//	backpackItems, err := getBackpackItemsFromRedis(p.Uid)
//	if err != nil {
//		slog.Error("Failed to get backpack items", "error", err)
//		p.SendResponse(msg, mustMarshal(&pb.BackpackItemsResponse{
//			Ret: pb.ErrorCode_SERVER_ERROR,
//		}))
//		return
//	}
//
//	// 发送响应
//	p.SendResponse(msg, mustMarshal(&pb.BackpackItemsResponse{
//		Ret:   pb.ErrorCode_OK,
//		Items: backpackItems,
//	}))
//}
//
//// 处理购买请求
//func handlePurchaseRequest(p *Player, msg *pb.Message) {
//	var req pb.PurchaseRequest
//	if err := proto.Unmarshal(msg.GetData(), &req); err != nil {
//		slog.Error("Failed to parse purchase request", "error", err)
//		p.SendResponse(msg, mustMarshal(&pb.PurchaseResponse{
//			Ret: pb.ErrorCode_INVALID_PARAM,
//		}))
//		return
//	}
//
//	// 获取商品信息
//	item, err := getMarketItem(req.ItemId)
//	if err != nil {
//		slog.Error("Failed to get market item", "item_id", req.ItemId, "error", err)
//		p.SendResponse(msg, mustMarshal(&pb.PurchaseResponse{
//			Ret: pb.ErrorCode_INVALID_ITEM,
//		}))
//		return
//	}
//
//	// 检查用户金币是否足够
//	if p.Gold < int64(item.Price) {
//		slog.Error("Insufficient gold", "uid", p.Uid, "gold", p.Gold, "price", item.Price)
//		p.SendResponse(msg, mustMarshal(&pb.PurchaseResponse{
//			Ret: pb.ErrorCode_INSUFFICIENT_GOLD,
//		}))
//		return
//	}
//
//	// 处理购买逻辑
//	err = processPurchase(p.Uid, req.ItemId, item.Price)
//	if err != nil {
//		slog.Error("Failed to process purchase", "error", err)
//		p.SendResponse(msg, mustMarshal(&pb.PurchaseResponse{
//			Ret: pb.ErrorCode_SERVER_ERROR,
//		}))
//		return
//	}
//
//	// 更新用户金币
//	p.Gold -= int64(item.Price)
//	p.saveUserData()
//
//	// 发送响应
//	p.SendResponse(msg, mustMarshal(&pb.PurchaseResponse{
//		Ret:    pb.ErrorCode_OK,
//		ItemId: req.ItemId,
//	}))
//}
//
//// 处理装备请求
//func handleEquipRequest(p *Player, msg *pb.Message) {
//	var req pb.EquipRequest
//	if err := proto.Unmarshal(msg.GetData(), &req); err != nil {
//		slog.Error("Failed to parse equip request", "error", err)
//		p.SendResponse(msg, mustMarshal(&pb.EquipResponse{
//			Ret: pb.ErrorCode_INVALID_PARAM,
//		}))
//		return
//	}
//
//	// 检查用户是否拥有该物品
//	if !p.hasItem(req.ItemId) {
//		slog.Error("User does not own item", "uid", p.Uid, "item_id", req.ItemId)
//		p.SendResponse(msg, mustMarshal(&pb.EquipResponse{
//			Ret: pb.ErrorCode_ITEM_NOT_OWNED,
//		}))
//		return
//	}
//
//	// 处理装备逻辑
//	err := p.equipItem(req.ItemId)
//	if err != nil {
//		slog.Error("Failed to equip item", "error", err)
//		p.SendResponse(msg, mustMarshal(&pb.EquipResponse{
//			Ret: pb.ErrorCode_SERVER_ERROR,
//		}))
//		return
//	}
//
//	// 发送响应
//	p.SendResponse(msg, mustMarshal(&pb.EquipResponse{
//		Ret:    pb.ErrorCode_OK,
//		ItemId: req.ItemId,
//	}))
//}
//
//// 处理获取皮肤数据请求
//func handleGetSkinDataRequest(p *Player, msg *pb.Message) {
//	var req pb.SkinDataRequest
//	if err := proto.Unmarshal(msg.GetData(), &req); err != nil {
//		slog.Error("Failed to parse skin data request", "error", err)
//		p.SendResponse(msg, mustMarshal(&pb.SkinDataResponse{
//			Ret: pb.ErrorCode_INVALID_PARAM,
//		}))
//		return
//	}
//
//	// 检查用户是否拥有该皮肤
//	if !p.hasItem(req.SkinId) {
//		slog.Error("User does not own skin", "uid", p.Uid, "skin_id", req.SkinId)
//		p.SendResponse(msg, mustMarshal(&pb.SkinDataResponse{
//			Ret: pb.ErrorCode_SKIN_NOT_OWNED,
//		}))
//		return
//	}
//
//	// 获取皮肤数据
//	skinData, err := getSkinData(req.SkinId)
//	if err != nil {
//		slog.Error("Failed to get skin data", "error", err)
//		p.SendResponse(msg, mustMarshal(&pb.SkinDataResponse{
//			Ret: pb.ErrorCode_SERVER_ERROR,
//		}))
//		return
//	}
//
//	// 发送响应
//	p.SendResponse(msg, mustMarshal(&pb.SkinDataResponse{
//		Ret:      pb.ErrorCode_OK,
//		SkinId:   req.SkinId,
//		SkinData: skinData,
//	}))
//}
//
//// 从Redis获取商城物品
//func getMarketItemsFromRedis() ([]*pb.SkinInfo, error) {
//	conn := db.Pool.Get()
//	defer conn.Close()
//
//	data, err := redis.String(conn.Do("GET", "market:items"))
//	if err != nil {
//		return nil, err
//	}
//
//	var items []*pb.SkinInfo
//	if err := json.Unmarshal([]byte(data), &items); err != nil {
//		return nil, err
//	}
//
//	return items, nil
//}
//
//// 从Redis获取用户背包物品
//func getBackpackItemsFromRedis(uid uint64) ([]*pb.SkinInfo, error) {
//	conn := db.Pool.Get()
//	defer conn.Close()
//
//	userKey := fmt.Sprintf("user:%d:backpack", uid)
//	data, err := redis.String(conn.Do("GET", userKey))
//	if err != nil {
//		return nil, err
//	}
//
//	var items []*pb.SkinInfo
//	if err := json.Unmarshal([]byte(data), &items); err != nil {
//		return nil, err
//	}
//
//	return items, nil
//}
//
//// 处理购买逻辑
//func processPurchase(uid uint64, itemId string, price int32) error {
//	conn := db.Pool.Get()
//	defer conn.Close()
//
//	// 使用事务确保原子性
//	_, err := conn.Do("MULTI")
//	if err != nil {
//		return err
//	}
//
//	// 扣除金币
//	_, err = conn.Do("HINCRBY", fmt.Sprintf("user:%d", uid), "gold", -int64(price))
//	if err != nil {
//		conn.Do("DISCARD")
//		return err
//	}
//
//	// 添加物品到背包
//	userBackpackKey := fmt.Sprintf("user:%d:backpack", uid)
//	_, err = conn.Do("SADD", userBackpackKey, itemId)
//	if err != nil {
//		conn.Do("DISCARD")
//		return err
//	}
//
//	// 执行事务
//	_, err = conn.Do("EXEC")
//	return err
//}
//
//// 获取皮肤数据
//func getSkinData(skinId string) ([]byte, error) {
//	conn := db.Pool.Get()
//	defer conn.Close()
//
//	// 从Redis获取加密的皮肤数据
//	data, err := redis.Bytes(conn.Do("GET", fmt.Sprintf("skin:%s:data", skinId)))
//	if err != nil {
//		return nil, err
//	}
//
//	return data, nil
//}
