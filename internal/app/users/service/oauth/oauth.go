package oauth

import (
	"HelpStudent/config"
	"HelpStudent/internal/app/users/model"
	"HelpStudent/internal/app/users/model/thirdPlat"
	"HelpStudent/internal/app/users/service/oauth/endpoint"
	"strings"

	"gorm.io/datatypes"
)

type Endpoint interface {
	Redirect(redirect string, state string) string
	Validate(code string, state string) (unionID string, attr datatypes.JSON, err error)
	GetUserName(attr datatypes.JSON) (userName string)
	GetUserStaffId(attr datatypes.JSON) (staffId string)
	GetUserAvatar(attr datatypes.JSON) (avatar string)
}

var platformMap = map[string]map[thirdPlat.Type]Endpoint{}

func Init() {
	for _, oAuth := range config.GetConfig().OAuth {
		platformMap[oAuth.CallbackURL] = map[thirdPlat.Type]Endpoint{
			thirdPlat.HDUHelp: &endpoint.HDUHelp{
				ClientID: oAuth.HDUHelp.ClientID, ClientSecret: oAuth.HDUHelp.ClientSecret,
			},
		}
	}
}

func PlatformExists(redirectUrl string, platform thirdPlat.Type) bool {
	for k, v := range platformMap {
		if strings.Index(redirectUrl, k) == 0 { // support domain prefix
			if _, ok := v[platform]; ok {
				return true
			}
		}
	}
	return false
}

func PlatformEndpoint(redirectUrl string, platform thirdPlat.Type) Endpoint {
	for k, v := range platformMap {
		if strings.Index(redirectUrl, k) == 0 { // support domain prefix
			if ep, ok := v[platform]; ok {
				return ep
			}
		}
	}
	return nil
}

func GetUserName(bind model.UserBind) string {
	for _, m := range platformMap {
		if e, ok := m[thirdPlat.FromString(bind.Type)]; ok {
			name := e.GetUserName(bind.Attr)
			if name != "" {
				return name
			}
		}
	}
	return ""
}

func GetStaffId(bind model.UserBind) string {
	for _, m := range platformMap {
		if e, ok := m[thirdPlat.FromString(bind.Type)]; ok {
			return e.GetUserStaffId(bind.Attr)
		}
	}
	return ""
}

func GetAvatar(bind model.UserBind) string {
	for _, m := range platformMap {
		if e, ok := m[thirdPlat.FromString(bind.Type)]; ok {
			return e.GetUserAvatar(bind.Attr)
		}
	}
	return ""
}
