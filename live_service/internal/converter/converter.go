package converter

import (
	"live_service/internal/model"
	livepb "live_service/proto/proto_gen"
)

// LiveStreamToProto 直播流转Proto
func LiveStreamToProto(stream *model.LiveStream) *livepb.LiveStream {
	if stream == nil {
		return nil
	}

	return &livepb.LiveStream{
		Id:          stream.ID,
		UserId:      stream.UserID,
		Title:       stream.Title,
		Description: stream.Description,
		CategoryId:  stream.CategoryID,
		Status:      string(stream.Status),
		StreamUrl:   stream.StreamURL,
		PlaybackUrl: stream.PlaybackURL,
		CoverImage:  stream.CoverImage,
		ViewerCount: stream.ViewerCount,
		LikeCount:   stream.LikeCount,
		GiftCount:   stream.GiftCount,
		GiftValue:   stream.GiftValue,
		Duration:    stream.Duration,
		StartTime:   stream.StartTime,
		EndTime:     stream.EndTime,
		CreatedAt:   stream.CreatedAt,
		UpdatedAt:   stream.UpdatedAt,
	}
}

// LiveStreamListToProto 直播流列表转Proto
func LiveStreamListToProto(streams []*model.LiveStream) []*livepb.LiveStream {
	if streams == nil {
		return nil
	}

	result := make([]*livepb.LiveStream, len(streams))
	for i, stream := range streams {
		result[i] = LiveStreamToProto(stream)
	}
	return result
}

// LiveRoomToProto 直播间转Proto
func LiveRoomToProto(room *model.LiveRoom) *livepb.LiveRoom {
	if room == nil {
		return nil
	}

	return &livepb.LiveRoom{
		Id:             room.ID,
		StreamId:       room.StreamID,
		Title:          room.Title,
		Description:    room.Description,
		CategoryId:     room.CategoryID,
		Status:         string(room.Status),
		ViewerCount:    room.ViewerCount,
		MaxViewerCount: room.MaxViewerCount,
		LikeCount:      room.LikeCount,
		GiftCount:      room.GiftCount,
		GiftValue:      room.GiftValue,
		ChatCount:      room.ChatCount,
		CreatedAt:      room.CreatedAt,
		UpdatedAt:      room.UpdatedAt,
	}
}

// LiveViewerToProto 直播观看者转Proto
func LiveViewerToProto(viewer *model.LiveViewer) *livepb.LiveViewer {
	if viewer == nil {
		return nil
	}

	return &livepb.LiveViewer{
		Id:         viewer.ID,
		StreamId:   viewer.StreamID,
		UserId:     viewer.UserID,
		UserName:   viewer.UserName,
		UserAvatar: viewer.UserAvatar,
		JoinTime:   viewer.JoinTime,
		LeaveTime:  viewer.LeaveTime,
		Duration:   viewer.Duration,
		IsMuted:    viewer.IsMuted,
		CreatedAt:  viewer.CreatedAt,
	}
}

// LiveViewerListToProto 直播观看者列表转Proto
func LiveViewerListToProto(viewers []*model.LiveViewer) []*livepb.LiveViewer {
	if viewers == nil {
		return nil
	}

	result := make([]*livepb.LiveViewer, len(viewers))
	for i, viewer := range viewers {
		result[i] = LiveViewerToProto(viewer)
	}
	return result
}

// LiveChatToProto 直播聊天转Proto
func LiveChatToProto(chat *model.LiveChat) *livepb.LiveChat {
	if chat == nil {
		return nil
	}

	return &livepb.LiveChat{
		Id:          chat.ID,
		StreamId:    chat.StreamID,
		UserId:      chat.UserID,
		UserName:    chat.UserName,
		UserAvatar:  chat.UserAvatar,
		Content:     chat.Content,
		ContentType: chat.ContentType,
		IsSystem:    chat.IsSystem,
		IsDeleted:   chat.IsDeleted,
		CreatedAt:   chat.CreatedAt,
	}
}

// LiveChatListToProto 直播聊天列表转Proto
func LiveChatListToProto(chats []*model.LiveChat) []*livepb.LiveChat {
	if chats == nil {
		return nil
	}

	result := make([]*livepb.LiveChat, len(chats))
	for i, chat := range chats {
		result[i] = LiveChatToProto(chat)
	}
	return result
}

// LiveGiftToProto 直播礼物转Proto
func LiveGiftToProto(gift *model.LiveGift) *livepb.LiveGift {
	if gift == nil {
		return nil
	}

	return &livepb.LiveGift{
		Id:         gift.ID,
		StreamId:   gift.StreamID,
		UserId:     gift.UserID,
		UserName:   gift.UserName,
		UserAvatar: gift.UserAvatar,
		GiftId:     gift.GiftID,
		GiftName:   gift.GiftName,
		GiftIcon:   gift.GiftIcon,
		GiftPrice:  gift.GiftPrice,
		GiftCount:  gift.GiftCount,
		TotalValue: gift.TotalValue,
		Message:    gift.Message,
		EffectType: gift.EffectType,
		CreatedAt:  gift.CreatedAt,
	}
}

// LiveGiftListToProto 直播礼物列表转Proto
func LiveGiftListToProto(gifts []*model.LiveGift) []*livepb.LiveGift {
	if gifts == nil {
		return nil
	}

	result := make([]*livepb.LiveGift, len(gifts))
	for i, gift := range gifts {
		result[i] = LiveGiftToProto(gift)
	}
	return result
}

// GiftConfigToProto 礼物配置转Proto
func GiftConfigToProto(config *service.GiftConfig) *livepb.GiftConfig {
	if config == nil {
		return nil
	}

	return &livepb.GiftConfig{
		Id:          config.ID,
		Name:        config.Name,
		Icon:        config.Icon,
		Price:       config.Price,
		CoinPrice:   config.CoinPrice,
		Category:    config.Category,
		Level:       config.Level,
		EffectType:  config.EffectType,
		EffectValue: config.EffectValue,
		Description: config.Description,
		IsActive:    config.IsActive,
		SortOrder:   config.SortOrder,
	}
}

// GiftConfigListToProto 礼物配置列表转Proto
func GiftConfigListToProto(configs []*service.GiftConfig) []*livepb.GiftConfig {
	if configs == nil {
		return nil
	}

	result := make([]*livepb.GiftConfig, len(configs))
	for i, config := range configs {
		result[i] = GiftConfigToProto(config)
	}
	return result
}

// LiveCategoryToProto 直播分类转Proto
func LiveCategoryToProto(category *service.LiveCategory) *livepb.LiveCategory {
	if category == nil {
		return nil
	}

	return &livepb.LiveCategory{
		Id:        category.ID,
		Name:      category.Name,
		Icon:      category.Icon,
		SortOrder: uint32(category.SortOrder),
		IsActive:  category.IsActive,
	}
}

// LiveCategoryListToProto 直播分类列表转Proto
func LiveCategoryListToProto(categories []*service.LiveCategory) []*livepb.LiveCategory {
	if categories == nil {
		return nil
	}

	result := make([]*livepb.LiveCategory, len(categories))
	for i, category := range categories {
		result[i] = LiveCategoryToProto(category)
	}
	return result
}

// LiveStatsToProto 直播统计转Proto
func LiveStatsToProto(stats *service.LiveStats) *livepb.LiveStats {
	if stats == nil {
		return nil
	}

	return &livepb.LiveStats{
		StreamId:       stats.StreamID,
		TotalViewers:   stats.TotalViewers,
		CurrentViewers: uint64(stats.CurrentViewers),
		MaxViewers:     uint64(stats.MaxViewers),
		LikeCount:      uint64(stats.LikeCount),
		GiftCount:      uint64(stats.GiftCount),
		CommentCount:   uint64(stats.CommentCount),
		ShareCount:     uint64(stats.ShareCount),
		Duration:       uint64(stats.Duration),
		GiftValue:      stats.GiftValue,
	}
}

// GiftRankingItemToProto 礼物排行榜项转Proto
func GiftRankingItemToProto(item *service.GiftRankingItem) *livepb.GiftRankingItem {
	if item == nil {
		return nil
	}

	return &livepb.GiftRankingItem{
		UserId:       item.UserID,
		UserName:     item.UserName,
		UserAvatar:   item.UserAvatar,
		GiftCount:    uint64(item.GiftCount),
		GiftValue:    item.GiftValue,
		Rank:         uint32(item.Rank),
		LastGiftTime: item.LastGiftTime,
	}
}

// GiftRankingListToProto 礼物排行榜列表转Proto
func GiftRankingListToProto(items []*service.GiftRankingItem) []*livepb.GiftRankingItem {
	if items == nil {
		return nil
	}

	result := make([]*livepb.GiftRankingItem, len(items))
	for i, item := range items {
		result[i] = GiftRankingItemToProto(item)
	}
	return result
}

// StartLiveRequestToModel 开始直播请求转Model
func StartLiveRequestToModel(req *livepb.StartLiveRequest, userID uint64) *model.LiveStream {
	if req == nil {
		return nil
	}

	return &model.LiveStream{
		UserID:      userID,
		Title:       req.Title,
		Description: req.Description,
		CategoryID:  req.CategoryId,
		Status:      model.LiveStatusPreparing,
	}
}

// SendLiveChatRequestToModel 发送聊天请求转Model
func SendLiveChatRequestToModel(req *livepb.SendLiveChatRequest, userID uint64) *model.LiveChat {
	if req == nil {
		return nil
	}

	return &model.LiveChat{
		StreamID:    req.StreamId,
		UserID:      userID,
		Content:     req.Content,
		ContentType: req.ContentType,
		IsSystem:    false,
		IsDeleted:   false,
	}
}

// SendLiveGiftRequestToModel 发送礼物请求转Model
func SendLiveGiftRequestToModel(req *livepb.SendLiveGiftRequest, userID uint64) *model.LiveGift {
	if req == nil {
		return nil
	}

	return &model.LiveGift{
		StreamID:  req.StreamId,
		UserID:    userID,
		GiftID:    req.GiftId,
		GiftCount: req.GiftCount,
		Message:   req.Message,
	}
}
