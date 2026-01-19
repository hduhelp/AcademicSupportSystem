package endpoint

import (
	"HelpStudent/core/logx"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/guonaihong/gout"
	"github.com/tidwall/gjson"
	"gorm.io/datatypes"
)

type HDUHelp struct {
	ClientID     string
	ClientSecret string
}

func (p *HDUHelp) Redirect(redirect string, state string) string {
	v := url.Values{}
	v.Add("response_type", "code")
	v.Add("client_id", p.ClientID)
	v.Add("redirect_uri", redirect)
	v.Add("state", state)
	re := strings.Builder{}
	re.WriteString("https://api.hduhelp.com/oauth/authorize?")
	re.WriteString(v.Encode())
	return re.String()
}

type HDUHelpStdResp struct {
	Error int             `json:"error"`
	Msg   string          `json:"msg"`
	Data  json.RawMessage `json:"data"`
}

type HDUHelpOAuthTokenResp struct {
	AccessToken        string `json:"access_token"`
	AccessTokenExpire  int    `json:"access_token_expire"`
	RefreshToken       string `json:"refresh_token"`
	RefreshTokenExpire int    `json:"refresh_token_expire"`
	StaffId            string `json:"staff_id"`
	StaffName          string `json:"staff_name"`
	StaffType          string `json:"staff_type"`
	UserId             string `json:"user_id"`
}

type HDUHelpPersonInfoResp struct {
	StaffId    string `json:"staffId"`
	StaffName  string `json:"staffName"`
	StaffState string `json:"staffState"`
	StaffType  string `json:"staffType"`
	UnitCode   string `json:"unitCode"`
}

type HDUHelpUserResp struct {
	Avatar string `json:"avatar"`
}

type HDUHelpAttr struct {
	HDUHelpOAuthTokenResp
	Avatar string `json:"avatar"`
}

func (p *HDUHelp) Validate(code string, state string) (staffId string, attr datatypes.JSON, err error) {
	var resp HDUHelpStdResp
	for i := 0; i < 3; i++ {
		err = gout.GET("https://api.hduhelp.com/oauth/token").
			SetQuery(gout.H{
				"client_id":     p.ClientID,
				"client_secret": p.ClientSecret,
				"grant_type":    "authorization_code",
				"code":          code,
				"state":         state,
			}).BindJSON(&resp).Do()
		if err == nil {
			break
		}
		if i != 0 {
			time.Sleep(100 * time.Millisecond)
		}
	}
	if err != nil {
		logx.SystemLogger.Errorf("HDUHelp OAuth error: %v", err)
		return
	}
	if resp.Error != 0 {
		return "", nil, fmt.Errorf("wrong code: %+v", resp)
	}

	tokenResp := HDUHelpOAuthTokenResp{}
	if err = json.Unmarshal(resp.Data, &tokenResp); err != nil {
		return
	}

	personInfoResp := HDUHelpPersonInfoResp{}
	if err = json.Unmarshal(resp.Data, &personInfoResp); err != nil {
		return
	}

	//获取头像
	for i := 0; i < 3; i++ {
		err = gout.GET("https://api.hduhelp.com/user/get").
			SetHeader(gout.H{
				"authorization": "token " + tokenResp.AccessToken,
			}).BindJSON(&resp).Do()
		if err == nil {
			break
		}
		if i != 0 {
			time.Sleep(100 * time.Millisecond)
		}
	}
	if err != nil {
		logx.SystemLogger.Errorf("HDUHelp OAuth get avatar error: %v", err)
		return
	}
	avatarResp := HDUHelpUserResp{}
	if err = json.Unmarshal(resp.Data, &avatarResp); err != nil {
		return
	}
	attr, _ = json.Marshal(HDUHelpAttr{
		HDUHelpOAuthTokenResp: tokenResp,
		Avatar:                avatarResp.Avatar,
	})
	return tokenResp.UserId, attr, nil
}

func (p *HDUHelp) GetUserName(attr datatypes.JSON) (userName string) {
	if nickName := gjson.GetBytes(attr, "staff_name"); nickName.Exists() && nickName.String() != "" {
		return nickName.String()
	}
	return ""
}

func (p *HDUHelp) GetUserStaffId(attr datatypes.JSON) (userName string) {
	if staffId := gjson.GetBytes(attr, "staff_id"); staffId.Exists() && staffId.String() != "" {
		return staffId.String()
	}
	logx.SystemLogger.Info(attr.String())
	return ""
}

func (p *HDUHelp) GetUserAvatar(attr datatypes.JSON) (avatar string) {
	if avatar := gjson.GetBytes(attr, "avatar"); avatar.Exists() && avatar.String() != "" {
		return avatar.String()
	}
	return ""
}
