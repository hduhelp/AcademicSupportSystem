package handler

import (
	"HelpStudent/core/auth"
	"HelpStudent/core/logx"
	"HelpStudent/core/middleware/response"
	"HelpStudent/internal/app/users/dao"
	"HelpStudent/internal/app/users/dto"
	"HelpStudent/internal/app/users/model"

	"github.com/flamego/flamego"
)

func HandleGetPersonInfo(r flamego.Render, c flamego.Context, auth auth.Info) {
	logx.SystemLogger.Infof("HandleGetPersonInfo: auth.Uid = %s, auth.StaffId = %s", auth.Uid, auth.StaffId)

	// 检查 DAO 是否已初始化
	if dao.Users == nil || dao.Users.DB == nil {
		logx.SystemLogger.Error("HandleGetPersonInfo: Users DAO 未初始化")
		response.ServiceErr(r, "服务未就绪，请稍后再试")
		return
	}

	var user model.Users
	result := dao.Users.WithContext(c.Request().Context()).Model(&model.Users{}).
		Where("id = ?", auth.Uid).Find(&user)

	userInfo := dto.UserInfoResponse{
		Id:          user.ID,
		StaffId:     user.StaffId,
		Name:        user.Name,
		Avatar:      user.Avatar,
		Permissions: nil,
	}

	if result.Error != nil {
		logx.SystemLogger.Errorf("HandleGetPersonInfo: 数据库链接错误: %v", result.Error)
		response.HTTPFail(r, 500, "Failed to get user info", result.Error)
		return
	} else {
		logx.SystemLogger.Info("HandleGetPersonInfo: success database connection")
	}

	if result.RowsAffected == 0 {
		logx.SystemLogger.Warnf("HandleGetPersonInfo: 找不到用户Uid %s", auth.Uid)
		response.HTTPFail(r, 404, "User not found", nil)
		return
	} else {
		logx.SystemLogger.Info("HandleGetPersonInfo: success find user info")
	}
	response.HTTPSuccess(r, userInfo)
}
