package conf

type Modules struct {
	ProxyAuth           bool
	Usage               bool
	StaticExchangeProbe bool
}

func NewModules(settings *Data, biz *Biz) Modules {
	repo := settings.GetRepo()
	modules := Modules{ProxyAuth: repo.GetProxyAuth() != nil, Usage: repo.GetUsage() != nil}
	modules.StaticExchangeProbe = (modules.ProxyAuth || modules.Usage) && biz.GetIpcrown().GetPasswordKey() != ""
	return modules
}
