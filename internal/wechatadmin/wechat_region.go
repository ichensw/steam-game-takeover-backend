package wechatadmin

import "strings"

func normalizeWechatRegion(country, province, city string) (string, string, string) {
	country = normalizeWechatCountry(country)
	province = normalizeWechatProvince(province)
	city = normalizeWechatCity(province, city)
	return country, province, city
}

func normalizeWechatCountry(value string) string {
	text := strings.TrimSpace(value)
	if text == "" || containsHan(text) {
		return text
	}
	if name, ok := wechatCountryNameCN[regionKey(text)]; ok {
		return name
	}
	return text
}

func normalizeWechatProvince(value string) string {
	text := strings.TrimSpace(value)
	if text == "" || containsHan(text) {
		return text
	}
	if name, ok := wechatProvinceNameCN[regionKey(text)]; ok {
		return name
	}
	return text
}

func normalizeWechatCity(province, value string) string {
	text := strings.TrimSpace(value)
	if text == "" || containsHan(text) {
		return text
	}
	if name, ok := wechatProvinceCityNameCN[regionKey(province)+"."+regionKey(text)]; ok {
		return name
	}
	if name, ok := wechatCityNameCN[regionKey(text)]; ok {
		return name
	}
	return text
}

func regionKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "", "-", "", "_", "", "'", "", ".", "")
	return replacer.Replace(value)
}

func containsHan(value string) bool {
	for _, r := range value {
		if r >= '\u4e00' && r <= '\u9fff' {
			return true
		}
	}
	return false
}

var wechatCountryNameCN = map[string]string{
	"cn":                    "中国",
	"china":                 "中国",
	"chinesemainland":       "中国大陆",
	"hk":                    "中国香港",
	"hongkong":              "中国香港",
	"mo":                    "中国澳门",
	"macau":                 "中国澳门",
	"macao":                 "中国澳门",
	"tw":                    "中国台湾",
	"taiwan":                "中国台湾",
	"us":                    "美国",
	"usa":                   "美国",
	"unitedstates":          "美国",
	"unitedstatesofamerica": "美国",
	"jp":                    "日本",
	"japan":                 "日本",
	"kr":                    "韩国",
	"korea":                 "韩国",
	"southkorea":            "韩国",
	"sg":                    "新加坡",
	"singapore":             "新加坡",
	"my":                    "马来西亚",
	"malaysia":              "马来西亚",
	"th":                    "泰国",
	"thailand":              "泰国",
	"gb":                    "英国",
	"uk":                    "英国",
	"unitedkingdom":         "英国",
	"ca":                    "加拿大",
	"canada":                "加拿大",
	"au":                    "澳大利亚",
	"australia":             "澳大利亚",
}

var wechatProvinceNameCN = map[string]string{
	"anhui":         "安徽",
	"beijing":       "北京",
	"chongqing":     "重庆",
	"fujian":        "福建",
	"gansu":         "甘肃",
	"guangdong":     "广东",
	"guangxi":       "广西",
	"guizhou":       "贵州",
	"hainan":        "海南",
	"hebei":         "河北",
	"heilongjiang":  "黑龙江",
	"henan":         "河南",
	"hubei":         "湖北",
	"hunan":         "湖南",
	"jiangsu":       "江苏",
	"jiangxi":       "江西",
	"jilin":         "吉林",
	"liaoning":      "辽宁",
	"neimenggu":     "内蒙古",
	"innermongolia": "内蒙古",
	"ningxia":       "宁夏",
	"qinghai":       "青海",
	"shaanxi":       "陕西",
	"shanxi":        "山西",
	"shandong":      "山东",
	"shanghai":      "上海",
	"sichuan":       "四川",
	"tianjin":       "天津",
	"tibet":         "西藏",
	"xizang":        "西藏",
	"xinjiang":      "新疆",
	"yunnan":        "云南",
	"zhejiang":      "浙江",
	"hongkong":      "香港",
	"macau":         "澳门",
	"macao":         "澳门",
	"taiwan":        "台湾",
}

var wechatProvinceCityNameCN = map[string]string{
	"guangdong.guangzhou": "广州",
	"guangdong.shaoguan":  "韶关",
	"guangdong.shenzhen":  "深圳",
	"guangdong.zhuhai":    "珠海",
	"guangdong.shantou":   "汕头",
	"guangdong.foshan":    "佛山",
	"guangdong.jiangmen":  "江门",
	"guangdong.zhanjiang": "湛江",
	"guangdong.maoming":   "茂名",
	"guangdong.zhaoqing":  "肇庆",
	"guangdong.huizhou":   "惠州",
	"guangdong.meizhou":   "梅州",
	"guangdong.shanwei":   "汕尾",
	"guangdong.heyuan":    "河源",
	"guangdong.yangjiang": "阳江",
	"guangdong.qingyuan":  "清远",
	"guangdong.dongguan":  "东莞",
	"guangdong.zhongshan": "中山",
	"guangdong.chaozhou":  "潮州",
	"guangdong.jieyang":   "揭阳",
	"guangdong.yunfu":     "云浮",
	"zhejiang.hangzhou":   "杭州",
	"zhejiang.ningbo":     "宁波",
	"zhejiang.wenzhou":    "温州",
	"zhejiang.jiaxing":    "嘉兴",
	"zhejiang.huzhou":     "湖州",
	"zhejiang.shaoxing":   "绍兴",
	"zhejiang.jinhua":     "金华",
	"zhejiang.quzhou":     "衢州",
	"zhejiang.zhoushan":   "舟山",
	"zhejiang.taizhou":    "台州",
	"zhejiang.lishui":     "丽水",
}

var wechatCityNameCN = map[string]string{
	"beijing":      "北京",
	"shanghai":     "上海",
	"tianjin":      "天津",
	"chongqing":    "重庆",
	"guangzhou":    "广州",
	"shenzhen":     "深圳",
	"shantou":      "汕头",
	"hangzhou":     "杭州",
	"ningbo":       "宁波",
	"wenzhou":      "温州",
	"suzhou":       "苏州",
	"nanjing":      "南京",
	"wuxi":         "无锡",
	"changzhou":    "常州",
	"xuzhou":       "徐州",
	"nantong":      "南通",
	"chengdu":      "成都",
	"wuhan":        "武汉",
	"xian":         "西安",
	"changsha":     "长沙",
	"qingdao":      "青岛",
	"jinan":        "济南",
	"fuzhou":       "福州",
	"xiamen":       "厦门",
	"kunming":      "昆明",
	"zhengzhou":    "郑州",
	"hefei":        "合肥",
	"nanchang":     "南昌",
	"nanning":      "南宁",
	"haikou":       "海口",
	"sanya":        "三亚",
	"shenyang":     "沈阳",
	"dalian":       "大连",
	"changchun":    "长春",
	"haerbin":      "哈尔滨",
	"harbin":       "哈尔滨",
	"taiyuan":      "太原",
	"shijiazhuang": "石家庄",
	"lanzhou":      "兰州",
	"yinchuan":     "银川",
	"xining":       "西宁",
	"urumqi":       "乌鲁木齐",
	"wulumuqi":     "乌鲁木齐",
	"lhasa":        "拉萨",
	"huhehaote":    "呼和浩特",
	"hohhot":       "呼和浩特",
	"hongkong":     "香港",
	"macau":        "澳门",
	"macao":        "澳门",
	"taipei":       "台北",
}
