package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/vision_world/video_service/internal/model"
	"github.com/vision_world/video_service/proto/proto_gen/user"
	"github.com/vision_world/video_service/proto/proto_gen/video"
	"gorm.io/gorm"
)

// CommentService 评论服务
type CommentService struct {
	db         *model.DB
	userClient user.UserServiceClient
}

// NewCommentService 创建评论服务实例
func NewCommentService(db *model.DB, userClient user.UserServiceClient) *CommentService {
	return &CommentService{
		db:         db,
		userClient: userClient,
	}
}

// SetUserClient 设置用户服务客户端
func (s *CommentService) SetUserClient(userClient user.UserServiceClient) {
	s.userClient = userClient
}

// CommentVideo 发表评论
func (s *CommentService) CommentVideo(ctx context.Context, req *video.CommentRequest) (*video.CommentResponse, error) {
	// 1. 验证请求参数
	if req.Content == "" {
		return &video.CommentResponse{
			StatusCode: 400,
			StatusMsg:  "评论内容不能为空",
		}, nil
	}

	// 2. 验证token并获取用户ID
	if req.Token == "" {
		return &video.CommentResponse{
			StatusCode: 401,
			StatusMsg:  "token不能为空",
		}, nil
	}

	// 调用用户服务验证token
	verifyResp, err := s.userClient.VerifyToken(ctx, &user.VerifyTokenRequest{Token: req.Token})
	if err != nil {
		return &video.CommentResponse{
			StatusCode: 401,
			StatusMsg:  "token无效",
		}, err
	}

	userID := verifyResp.UserId
	log.Println(userID)

	// 4. 创建评论
	comment := &model.VideoComment{
		VideoID:   req.VideoId,
		UserID:    userID,
		ParentID:  req.ParentId,
		Content:   req.Content,
		LikeCount: 0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 5. 保存到数据库
	result := s.db.Create(comment)
	if result.Error != nil {
		return &video.CommentResponse{
			StatusCode: 500,
			StatusMsg:  "发表评论失败",
		}, result.Error
	}

	// 6. 获取用户信息
	var userInfo *user.User
	if s.userClient != nil {
		userResp, err := s.userClient.GetUserInfo(ctx, &user.GetUserInfoRequest{
			UserId: userID,
			Token:  req.Token,
		})
		if err == nil && userResp.StatusCode == 0 && userResp.User != nil {
			userInfo = userResp.User
		}
	}

	// 7. 获取回复用户信息
	var replyToUserInfo *user.User
	if comment.ReplyToUserID != nil && s.userClient != nil {
		userResp, err := s.userClient.GetUserInfo(ctx, &user.GetUserInfoRequest{
			UserId: *comment.ReplyToUserID,
			Token:  req.Token,
		})
		if err == nil && userResp.StatusCode == 0 && userResp.User != nil {
			replyToUserInfo = userResp.User
		}
	}

	// 8. 返回响应
	return &video.CommentResponse{
		StatusCode: 0,
		StatusMsg:  "发表评论成功",
		Comment: &video.Comment{
			Id:          comment.ID,
			User:        userInfo,
			Content:     comment.Content,
			VideoId:     comment.VideoID,
			ParentId:    comment.ParentID,
			ReplyToUser: replyToUserInfo,
			LikeCount:   comment.LikeCount,
			CreateTime:  time.Now().Unix(),
			IsLiked:     false,
		},
	}, nil
}

// DeleteComment 删除评论
func (s *CommentService) DeleteComment(ctx context.Context, req *video.DeleteCommentRequest) (*video.DeleteCommentResponse, error) {
	// 1. 验证token并获取用户ID
	if req.Token == "" {
		return &video.DeleteCommentResponse{
			StatusCode: 401,
			StatusMsg:  "token不能为空",
		}, nil
	}

	// 调用用户服务验证token
	verifyResp, err := s.userClient.VerifyToken(ctx, &user.VerifyTokenRequest{Token: req.Token})
	if err != nil || verifyResp.StatusCode != 0 || !verifyResp.Valid {
		return &video.DeleteCommentResponse{
			StatusCode: 401,
			StatusMsg:  "token无效",
		}, err
	}

	userID := verifyResp.UserId

	// 2. 检查评论是否存在
	var comment model.VideoComment
	result := s.db.First(&comment, req.CommentId)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return &video.DeleteCommentResponse{
				StatusCode: 404,
				StatusMsg:  "评论不存在",
			}, nil
		}
		return &video.DeleteCommentResponse{
			StatusCode: 500,
			StatusMsg:  "删除评论失败",
		}, result.Error
	}

	// 3. 检查用户是否有权限删除评论
	if comment.UserID != userID {
		return &video.DeleteCommentResponse{
			StatusCode: 403,
			StatusMsg:  "无权删除该评论",
		}, nil
	}

	// 4. 删除评论（软删除）
	result = s.db.Delete(&comment)
	if result.Error != nil {
		return &video.DeleteCommentResponse{
			StatusCode: 500,
			StatusMsg:  "删除评论失败",
		}, result.Error
	}

	// 5. 返回响应
	return &video.DeleteCommentResponse{
		StatusCode: 0,
		StatusMsg:  "删除评论成功",
	}, nil
}

// GetVideoComments 获取视频评论列表
func (s *CommentService) GetVideoComments(ctx context.Context, req *video.GetVideoCommentsRequest) (*video.GetVideoCommentsResponse, error) {
	// 1. 设置默认值
	page := req.Page
	if page <= 0 {
		page = 1
	}
	log.Println("进入获取视频评论列表")
	pageSize := req.PageSize
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 10
	}

	// 2. 计算偏移量
	offset := (page - 1) * pageSize

	// 3. 查询评论列表
	var comments []model.VideoComment
	var total int64

	// 查询总数
	s.db.Model(&model.VideoComment{}).Where("video_id = ? AND parent_id IS NULL", req.VideoId).Count(&total)

	// 查询评论列表
	query := s.db.Where("video_id = ?", req.VideoId)
	if req.SortOrder == "hot" {
		query = query.Order("like_count DESC")
	} else {
		query = query.Order("created_at DESC")
	}

	result := query.Offset(int(offset)).Limit(int(pageSize)).Find(&comments)
	if result.Error != nil {
		log.Println("查询评论db报错了")
		log.Println(result.Error)
		return &video.GetVideoCommentsResponse{
			StatusCode: 500,
			StatusMsg:  "获取评论列表失败",
		}, result.Error
	}

	// 4. 收集所有需要查询的用户ID
	allUserIDs := make(map[uint32]bool)
	for _, comment := range comments {
		allUserIDs[comment.UserID] = true
		if comment.ReplyToUserID != nil {
			allUserIDs[*comment.ReplyToUserID] = true
		}

		var subComments []model.VideoComment
		s.db.Where("parent_id = ?", comment.ID).Order("created_at DESC").Find(&subComments)
		for _, subComment := range subComments {
			allUserIDs[subComment.UserID] = true
			if subComment.ReplyToUserID != nil {
				allUserIDs[*subComment.ReplyToUserID] = true
			}
		}
	}

	// 5. 批量获取用户信息
	userMap := make(map[uint32]*user.User)
	if s.userClient != nil && len(allUserIDs) > 0 {
		userIDList := make([]uint32, 0, len(allUserIDs))
		for id := range allUserIDs {
			userIDList = append(userIDList, id)
		}

		userResp, err := s.userClient.GetUserInfos(ctx, &user.GetUserInfosRequest{
			UserIds: userIDList,
			Token:   req.Token,
		})
		log.Println("拿到的用户信息")
		log.Println(userResp)
		if err == nil && userResp.StatusCode == 0 {
			for _, u := range userResp.Users {
				userMap[u.Id] = u
			}
		}
	}

	// 6. 转换为proto格式
	var commentList []*video.Comment
	for _, comment := range comments {
		log.Println("单条评论获取成功")

		var subComments []model.VideoComment
		s.db.Where("parent_id = ?", comment.ID).Order("created_at DESC").Find(&subComments)

		var subCommentList []*video.Comment
		for _, subComment := range subComments {
			log.Println("单条子评论获取成功")
			subCommentList = append(subCommentList, &video.Comment{
				Id:         subComment.ID,
				User:       userMap[subComment.UserID],
				VideoId:    subComment.VideoID,
				Content:    subComment.Content,
				ParentId:   subComment.ParentID,
				LikeCount:  subComment.LikeCount,
				CreateTime: subComment.CreatedAt.Unix(),
				IsLiked:    false,
			})
			if subComment.ReplyToUserID != nil {
				subCommentList[len(subCommentList)-1].ReplyToUser = userMap[*subComment.ReplyToUserID]
			}
		}

		var replyToUser *user.User
		if comment.ReplyToUserID != nil {
			replyToUser = userMap[*comment.ReplyToUserID]
		}

		commentList = append(commentList, &video.Comment{
			Id:          comment.ID,
			User:        userMap[comment.UserID],
			VideoId:     comment.VideoID,
			Content:     comment.Content,
			ParentId:    comment.ParentID,
			ReplyToUser: replyToUser,
			LikeCount:   comment.LikeCount,
			CreateTime:  comment.CreatedAt.Unix(),
			IsLiked:     false,
			Replies:     subCommentList,
		})
	}
	log.Println("成功了")
	// 5. 返回响应
	return &video.GetVideoCommentsResponse{
		StatusCode: 0,
		StatusMsg:  "获取评论列表成功",
		Comments:   commentList,
		Total:      uint32(total),
		HasMore:    uint32(len(commentList)) == pageSize,
	}, nil
}

// LikeComment 点赞评论
func (s *CommentService) LikeComment(ctx context.Context, req *video.LikeCommentRequest) (*video.LikeCommentResponse, error) {
	// 1. 解析用户ID（实际应用中应该从token中解析）
	// 这里暂时硬编码为1，实际应用中应该从token中解析
	userID := uint32(1)

	// 2. 检查评论是否存在
	var comment model.VideoComment
	result := s.db.First(&comment, req.CommentId)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return &video.LikeCommentResponse{
				StatusCode: 404,
				StatusMsg:  "评论不存在",
			}, nil
		}
		return &video.LikeCommentResponse{
			StatusCode: 500,
			StatusMsg:  "点赞失败",
		}, result.Error
	}

	// 3. 检查是否已经点赞
	var commentLike model.VideoCommentLike
	result = s.db.Where("comment_id = ? AND user_id = ?", req.CommentId, userID).First(&commentLike)

	if req.ActionType {
		// 点赞
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				// 未点赞，添加点赞记录
				commentLike = model.VideoCommentLike{
					CommentID: req.CommentId,
					UserID:    userID,
					CreatedAt: time.Now(),
				}
				s.db.Create(&commentLike)

				// 点赞数+1
				s.db.Model(&comment).Update("like_count", comment.LikeCount+1)
				comment.LikeCount++
			}
		}
	} else {
		// 取消点赞
		if result.Error == nil {
			// 已点赞，删除点赞记录
			s.db.Delete(&commentLike)

			// 点赞数-1
			if comment.LikeCount > 0 {
				s.db.Model(&comment).Update("like_count", comment.LikeCount-1)
				comment.LikeCount--
			}
		}
	}

	// 4. 返回响应
	return &video.LikeCommentResponse{
		StatusCode: 0,
		StatusMsg:  "操作成功",
		LikeCount:  comment.LikeCount,
		IsLiked:    req.ActionType,
	}, nil
}

// getUserInfo 获取用户信息
func (s *CommentService) getUserInfo(ctx context.Context, userID uint32) (*user.User, error) {
	log.Println("获取用户信息")
	log.Println(userID)
	resp, err := s.userClient.GetUserInfo(ctx, &user.GetUserInfoRequest{
		UserId: userID,
	})
	log.Println(resp)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 0 {
		return nil, fmt.Errorf("user service returned error: %s", resp.StatusMsg)
	}

	if resp.User == nil {
		return nil, fmt.Errorf("user service returned nil user for userID: %d", userID)
	}

	userInfo := resp.User

	return userInfo, nil
}
