package converter

import (
	"time"

	"google.golang.org/protobuf/types/known/wrapperspb"
	"user_service/internal/model"
	"user_service/proto/proto_gen"
)

// UserConverter 用户模型转换器
type UserConverter struct{}

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
		FollowCount:     wrapperspb.UInt32(user.FollowingCount),
		FollowerCount:   wrapperspb.UInt32(user.FollowersCount),
		IsFollow:        false, // 默认未关注，实际业务中需要根据上下文判断
		Avatar:          &user.AvatarURL,
		BackgroundImage: &user.BackgroundImage,
		Signature:       &user.Signature,
		TotalFavorited:  wrapperspb.UInt32(uint32(user.TotalFavorited)),
		WorkCount:       wrapperspb.UInt32(user.WorkCount),
		FavoriteCount:   wrapperspb.UInt32(user.FavoriteCount),
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
		AvatarURL:       protoUser.Avatar,
		BackgroundImage: protoUser.BackgroundImage,
		Signature:       protoUser.Signature,
		IsVerified:      protoUser.IsVerified,
		UserType:        protoUser.UserType,
	}

	// 处理optional字段
	if protoUser.FollowCount != nil {
		user.FollowingCount = protoUser.FollowCount.Value
	}
	if protoUser.FollowerCount != nil {
		user.FollowersCount = protoUser.FollowerCount.Value
	}
	if protoUser.TotalFavorited != nil {
		user.TotalFavorited = uint64(protoUser.TotalFavorited.Value)
	}
	if protoUser.WorkCount != nil {
		user.WorkCount = protoUser.WorkCount.Value
	}
	if protoUser.FavoriteCount != nil {
		user.FavoriteCount = protoUser.FavoriteCount.Value
	}

	return user
}
