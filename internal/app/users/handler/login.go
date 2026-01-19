package handler

import (
	"HelpStudent/config"
	"HelpStudent/core/auth"
	"HelpStudent/core/cache"
	"HelpStudent/core/logx"
	"HelpStudent/core/middleware/response"
	"HelpStudent/core/store/rds"
	managersDao "HelpStudent/internal/app/managers/dao"
	"HelpStudent/internal/app/users/dao"
	"HelpStudent/internal/app/users/dto"
	"HelpStudent/internal/app/users/model"
	"HelpStudent/internal/app/users/model/thirdPlat"
	"HelpStudent/internal/app/users/service/oauth"
	"HelpStudent/pkg/utils"
	"strings"
	"time"

	"github.com/flamego/binding"
	"github.com/flamego/flamego"
	"gorm.io/datatypes"
)

func HandleThirdPlatLogin(r flamego.Render, c flamego.Context) {
	req := dto.ThirdPlatLoginReq{
		Callback: c.Query("callback"),
		Platform: c.Query("platform"),
		From:     c.Query("from"),
	}

	if req.Callback == "" || req.Platform == "" || req.From == "" {
		response.HTTPFail(r, 401001, "参数错误")
		return
	}

	var callbackExist bool
	for _, oAuth := range config.GetConfig().OAuth {
		if oAuth.CallbackURL == req.Callback || (strings.Index(req.Callback, oAuth.CallbackURL) == 0) {
			callbackExist = true
			break
		}
	}
	if !callbackExist {
		response.HTTPFail(r, 401002, "回调地址不合法")
		return
	}

	platType := thirdPlat.FromString(req.Platform)
	if platType == thirdPlat.NotExists {
		response.HTTPFail(r, 401001, "平台暂不支持")
		return
	}

	urlParams := map[string][]string{}
	if req.From != "" {
		urlParams["from"] = []string{req.From}
	}
	callbackUrl := utils.UrlAppend(req.Callback, urlParams)
	var redirectURL, mark string
	if oauth.PlatformExists(req.Callback, platType) {
		redirectURL, mark = oauth.GetRedirectUrl(req.Callback, platType, callbackUrl)
	} else {
		response.HTTPFail(r, 401001, "平台暂不支持")
		return
	}
	err := cache.Setex(rds.Key("oauth", "mark", mark), "", 60*15)
	if err != nil {
		response.ServiceErr(r, err)
		return
	}

	response.HTTPSuccess(r, dto.ThirdPlatLoginResp{
		URL: redirectURL,
	})
}

func HandleThirdPlatCallback(r flamego.Render, c flamego.Context, req dto.ThirdPlatLoginCallbackReq, errs binding.Errors) {
	if errs != nil {
		response.InValidParam(r, errs)
		return
	}

	if req.Code == "" && req.State == "" {
		response.HTTPFail(r, 401001, "code和state不能同时为空")
		return
	}

	states := strings.Split(req.State, "_")
	// 校验state格式 platform_mark
	if len(states) != 2 {
		response.HTTPFail(r, 401001, "state格式错误")
		return
	}

	var callbackExist bool
	for _, oAuth := range config.GetConfig().OAuth {
		if oAuth.CallbackURL == req.Callback || (strings.Index(req.Callback, oAuth.CallbackURL) == 0) {
			callbackExist = true
			break
		}
	}
	if !callbackExist {
		response.HTTPFail(r, 401002, "回调地址不合法")
		return
	}

	platType := thirdPlat.FromString(states[0])
	if platType == thirdPlat.NotExists {
		response.HTTPFail(r, 401001, "平台暂不支持")
		return
	}

	mark := states[1]
	if mark == "" {
		response.HTTPFail(r, 401001, "mark不能为空")
		return
	}

	if exist, err := cache.ExistsCtx(c.Request().Context(), rds.Key("oauth", "mark", mark)); !exist || err != nil {
		response.HTTPFail(r, 401001, "mark已失效")
		return
	}
	_, err := cache.DelCtx(c.Request().Context(), rds.Key("oauth", "mark", mark))
	if err != nil {
		logx.ServiceLogger.CtxError(c.Request().Context(), err)
	}

	var (
		uid  string
		attr datatypes.JSON
	)
	if oauth.PlatformExists(req.Callback, platType) {
		uid, attr, err = oauth.Validate(req.Callback, platType, req.Code, req.State)
	} else {
		response.HTTPFail(r, 401001, "平台暂不支持")
		return
	}
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.ServiceErr(r, err)
		return
	}
	b := &model.UserBind{Type: platType.String(), UnionId: uid}
	if result := dao.Users.WithContext(c.Request().Context()).Where(b).
		Find(b); result.Error != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), result.Error)
		response.ServiceErr(r, result.Error)
		return
	} else if result.RowsAffected == 0 {
		// 新用户
		b.Attr = attr
		user := &model.Users{
			StaffId: oauth.GetStaffId(*b),
			Name:    oauth.GetUserName(*b),
			Avatar:  oauth.GetAvatar(*b),
		}
		err = dao.Users.CreateWithBind(c.Request().Context(), user, b)
		if err != nil {
			logx.SystemLogger.CtxError(c.Request().Context(), err)
			response.ServiceErr(r, err)
			return
		}
		// CreateWithBind 会设置 b.UserId，但如果用户已存在需要重新查询
		if b.UserId == "" {
			b.UserId = user.ID
		}
	} else {
		// 老用户更新用户信息
		b.Attr = attr
		dao.Users.WithContext(c.Request().Context()).Model(b).Update("attr", attr)
	}

	token, err := auth.GenToken(auth.Info{Uid: b.UserId, StaffId: oauth.GetStaffId(*b), Name: oauth.GetUserName(*b)})
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.ServiceErr(r, err)
		return
	}
	refreshToken, err := auth.GenToken(auth.Info{Uid: b.UserId, StaffId: oauth.GetStaffId(*b), Name: oauth.GetUserName(*b), IsRefreshToken: true}, auth.RefreshTokenExpireIn)
	if err != nil {
		logx.SystemLogger.CtxError(c.Request().Context(), err)
		response.ServiceErr(r, err)
		return
	}

	// 检查是否是管理员
	staffId := oauth.GetStaffId(*b)
	name := oauth.GetUserName(*b)
	isManager := managersDao.Managers.IsManager(staffId)

	response.HTTPSuccess(r, dto.ThirdPlatLoginCallbackResp{
		AccessToken:          token,
		AccessTokenExpireIn:  int64(auth.AccessTokenExpireIn / time.Second),
		RefreshToken:         refreshToken,
		RefreshTokenExpireIn: int64(auth.RefreshTokenExpireIn / time.Second),
		IsManager:            isManager,
		StaffId:              staffId,
		Name:                 name,
	})
}

func HandleRefreshToken(r flamego.Render, req dto.RefreshTokenRequest) {
	entity, err := auth.ParseToken(req.RefreshToken)
	if err != nil || !entity.Info.IsRefreshToken || entity.Info.Name == "" || entity.Info.StaffId == "" {
		response.UnAuthorization(r)
		return
	}

	token, err := auth.GenToken(auth.Info{Uid: entity.Info.Uid, StaffId: entity.Info.StaffId, Name: entity.Info.Name})
	if err != nil {
		response.ServiceErr(r, err)
		return
	}
	response.HTTPSuccess(r, dto.RefreshTokenResponse{
		AccessToken:         token,
		AccessTokenExpireIn: int64(auth.AccessTokenExpireIn / time.Second),
		RefreshToken:        req.RefreshToken,
	})
}
