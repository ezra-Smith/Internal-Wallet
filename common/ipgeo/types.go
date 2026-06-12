package ipgeo

// GeoLocation 统一的IP地理位置信息结构
type GeoLocation struct {
	// IP 查询的IP地址
	IP string `json:"ip"`

	// Country 国家（中文）
	Country string `json:"country"`

	// Province 省份/州（中文）
	Province string `json:"province"`

	// City 城市（中文）
	City string `json:"city"`

	// ISP 运营商（可选）
	ISP string `json:"isp,omitempty"`

	// Source 数据来源 (pconline/ip-api)
	Source string `json:"source"`
}

// pconlineResponse 太平洋IP查询响应结构
type pconlineResponse struct {
	IP          string `json:"ip"`
	Pro         string `json:"pro"`         // 省份
	City        string `json:"city"`        // 城市
	Addr        string `json:"addr"`        // 详细地址
	Region      string `json:"region"`      // 地区
	RegionNames string `json:"regionNames"` // 地区名称
	Err         string `json:"err"`         // 错误信息
}

// ipAPIResponse ip-api.com 查询响应结构
type ipAPIResponse struct {
	Status      string  `json:"status"`      // success/fail
	Message     string  `json:"message"`     // 错误消息（当status=fail时）
	Country     string  `json:"country"`     // 国家
	CountryCode string  `json:"countryCode"` // 国家代码
	Region      string  `json:"region"`      // 地区代码
	RegionName  string  `json:"regionName"`  // 地区名称（省份/州）
	City        string  `json:"city"`        // 城市
	Zip         string  `json:"zip"`         // 邮编
	Lat         float64 `json:"lat"`         // 纬度
	Lon         float64 `json:"lon"`         // 经度
	Timezone    string  `json:"timezone"`    // 时区
	ISP         string  `json:"isp"`         // 运营商
	Org         string  `json:"org"`         // 组织
	AS          string  `json:"as"`          // AS号
	Query       string  `json:"query"`       // 查询的IP
}
