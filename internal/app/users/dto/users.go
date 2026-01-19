package dto

type UserInfoResponse struct {
	Id          string   `json:"id"`
	StaffId     string   `json:"staffId"`
	Name        string   `json:"name"`
	Avatar      string   `json:"avatar"`
	Permissions []string `json:"permissions" gorm:"-"`
}
