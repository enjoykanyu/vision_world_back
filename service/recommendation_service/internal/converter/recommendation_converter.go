package converter

import (
	"time"

	"recommendation_service/internal/model"
	"recommendation_service/proto/proto_gen"
)

// UserConverter 用户模型转换器
type UserConverter struct{}

// RecommendationConverter 推荐服务转换器
type RecommendationConverter struct{}

// NewUserConverter 创建用户转换器
func NewUserConverter() *UserConverter {
	return &UserConverter{}
}

// ModelToProto 将数据库模型User转换为protobuf User
func (c *UserConverter) ModelToProto(user *model.User) *proto_gen.User {
	if user == nil {
		return nil
	}

	// 手机号脱敏处理
	maskedPhone := c.maskPhone(user.Phone)

	// 处理时间戳
	createTime := c.getTimestamp(user.CreatedAt)
	lastLoginTime := c.getTimestampPtr(user.LastLoginAt)

	return &proto_gen.User{
		Id:              user.ID,
		Name:            user.Nickname, // 使用昵称作为显示名称
		Phone:           maskedPhone,
		FollowCount:     &user.FollowingCount,
		FollowerCount:   &user.FollowersCount,
		IsFollow:        false, // 默认未关注，实际业务中需要根据上下文判断
		Avatar:          &user.AvatarURL,
		BackgroundImage: &user.BackgroundImage,
		Signature:       &user.Signature,
		TotalFavorited:  &[]uint32{uint32(user.TotalFavorited)}[0],
		WorkCount:       &user.WorkCount,
		FavoriteCount:   &user.FavoriteCount,
		CreateTime:      createTime,
		LastLoginTime:   lastLoginTime,
		IsVerified:      user.IsVerified,
		UserType:        user.UserType,
	}
}

// ModelToProtoWithFollow 将数据库模型User转换为protobuf User（包含关注状态）
func (c *UserConverter) ModelToProtoWithFollow(user *model.User, isFollow bool) *proto_gen.User {
	if user == nil {
		return nil
	}

	protoUser := c.ModelToProto(user)
	protoUser.IsFollow = isFollow
	return protoUser
}

// ModelListToProtoList 将数据库模型User列表转换为protobuf User列表
func (c *UserConverter) ModelListToProtoList(users []*model.User) []*proto_gen.User {
	if users == nil || len(users) == 0 {
		return []*proto_gen.User{}
	}

	protoUsers := make([]*proto_gen.User, len(users))
	for i, user := range users {
		protoUsers[i] = c.ModelToProto(user)
	}
	return protoUsers
}

// ModelListToProtoListWithFollow 将数据库模型User列表转换为protobuf User列表（包含关注状态）
func (c *UserConverter) ModelListToProtoListWithFollow(users []*model.User, isFollowList []bool) []*proto_gen.User {
	if users == nil || len(users) == 0 {
		return []*proto_gen.User{}
	}

	protoUsers := make([]*proto_gen.User, len(users))
	for i, user := range users {
		if i < len(isFollowList) {
			protoUsers[i] = c.ModelToProtoWithFollow(user, isFollowList[i])
		} else {
			protoUsers[i] = c.ModelToProto(user)
		}
	}
	return protoUsers
}

// maskPhone 手机号脱敏处理
func (c *UserConverter) maskPhone(phone string) string {
	if phone != "" && len(phone) >= 11 {
		return phone[:3] + "****" + phone[7:]
	}
	return phone
}

// getTimestamp 获取时间戳
func (c *UserConverter) getTimestamp(t time.Time) int64 {
	return t.Unix()
}

// getTimestampPtr 获取时间戳指针
func (c *UserConverter) getTimestampPtr(t *time.Time) int64 {
	if t != nil {
		return t.Unix()
	}
	return 0
}

// ProtoToModel 将protobuf User转换为数据库模型User（用于更新操作）
func (c *UserConverter) ProtoToModel(protoUser *proto_gen.User) *model.User {
	if protoUser == nil {
		return nil
	}

	user := &model.User{
		ID:              protoUser.Id,
		Nickname:        protoUser.Name,
		Phone:           protoUser.Phone,
		AvatarURL:       protoUser.GetAvatar(),
		BackgroundImage: protoUser.GetBackgroundImage(),
		Signature:       protoUser.GetSignature(),
		IsVerified:      protoUser.IsVerified,
		UserType:        protoUser.UserType,
	}

	// 处理optional字段
	if protoUser.FollowCount != nil {
		user.FollowingCount = *protoUser.FollowCount
	}
	if protoUser.FollowerCount != nil {
		user.FollowersCount = *protoUser.FollowerCount
	}
	if protoUser.TotalFavorited != nil {
		user.TotalFavorited = uint64(*protoUser.TotalFavorited)
	}
	if protoUser.WorkCount != nil {
		user.WorkCount = *protoUser.WorkCount
	}
	if protoUser.FavoriteCount != nil {
		user.FavoriteCount = *protoUser.FavoriteCount
	}

	return user
}

// NewRecommendationConverter 创建推荐服务转换器
func NewRecommendationConverter() *RecommendationConverter {
	return &RecommendationConverter{}
}

// ModelToProto 将视频模型转换为proto
func (c *RecommendationConverter) ModelToProto(video *model.Video) *proto_gen.Video {
	if video == nil {
		return nil
	}

	return &proto_gen.Video{
		VideoId:     video.VideoID,
		Title:       video.Title,
		Description: video.Description,
		Author:      video.Author,
		Category:    video.Category,
		Tags:        video.Tags,
		Duration:    video.Duration,
		CoverUrl:    video.CoverURL,
		PlayUrl:     video.PlayURL,
		ViewCount:   video.ViewCount,
		LikeCount:   video.LikeCount,
		Score:       video.Score,
		CreatedAt:   video.CreatedAt.Unix(),
		UpdatedAt:   video.UpdatedAt.Unix(),
	}
}

// ProtoToModel 将proto转换为视频模型
func (c *RecommendationConverter) ProtoToModel(protoVideo *proto_gen.Video) *model.Video {
	if protoVideo == nil {
		return nil
	}

	return &model.Video{
		VideoID:     protoVideo.VideoId,
		Title:       protoVideo.Title,
		Description: protoVideo.Description,
		Author:      protoVideo.Author,
		Category:    protoVideo.Category,
		Tags:        protoVideo.Tags,
		Duration:    protoVideo.Duration,
		CoverURL:    protoVideo.CoverUrl,
		PlayURL:     protoVideo.PlayUrl,
		ViewCount:   protoVideo.ViewCount,
		LikeCount:   protoVideo.LikeCount,
		Score:       protoVideo.Score,
	}
}

// ModelToProtoUserPreference 将用户偏好模型转换为proto
func (c *RecommendationConverter) ModelToProtoUserPreference(preference *model.UserPreference) *proto_gen.UserPreference {
	if preference == nil {
		return nil
	}

	return &proto_gen.UserPreference{
		UserId:          preference.UserID,
		Categories:      preference.Categories,
		Tags:            preference.Tags,
		CategoryWeights: preference.CategoryWeights,
		TagWeights:      preference.TagWeights,
		CreatedAt:       preference.CreatedAt.Unix(),
		UpdatedAt:       preference.UpdatedAt.Unix(),
	}
}

// ProtoToModelUserPreference 将proto转换为用户偏好模型
func (c *RecommendationConverter) ProtoToModelUserPreference(protoPreference *proto_gen.UserPreference) *model.UserPreference {
	if protoPreference == nil {
		return nil
	}

	return &model.UserPreference{
		UserID:          protoPreference.UserId,
		Categories:      protoPreference.Categories,
		Tags:            protoPreference.Tags,
		CategoryWeights: protoPreference.CategoryWeights,
		TagWeights:      protoPreference.TagWeights,
	}
}

// ModelToProtoUserAction 将用户行为模型转换为proto
func (c *RecommendationConverter) ModelToProtoUserAction(action *model.UserAction) *proto_gen.UserAction {
	if action == nil {
		return nil
	}

	return &proto_gen.UserAction{
		ActionId:      action.ActionID,
		UserId:        action.UserID,
		VideoId:       action.VideoID,
		ActionType:    action.ActionType,
		Duration:      action.Duration,
		TotalDuration: action.TotalDuration,
		Timestamp:     action.Timestamp,
		CreatedAt:     action.CreatedAt.Unix(),
	}
}

// ProtoToModelUserAction 将proto转换为用户行为模型
func (c *RecommendationConverter) ProtoToModelUserAction(protoAction *proto_gen.UserAction) *model.UserAction {
	if protoAction == nil {
		return nil
	}

	return &model.UserAction{
		ActionID:      protoAction.ActionId,
		UserID:        protoAction.UserId,
		VideoID:       protoAction.VideoId,
		ActionType:    protoAction.ActionType,
		Duration:      protoAction.Duration,
		TotalDuration: protoAction.TotalDuration,
		Timestamp:     protoAction.Timestamp,
	}
}
