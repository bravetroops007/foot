package proc

import (
	"encoding/json"
	"github.com/PuerkitoBio/goquery"
	"github.com/hu17889/go_spider/core/common/page"
	"github.com/hu17889/go_spider/core/pipeline"
	"github.com/hu17889/go_spider/core/spider"
	"tesou.io/platform/foot-parent/foot-api/common/base"
	"tesou.io/platform/foot-parent/foot-spider/module/win007/down"
	"regexp"
	"strconv"
	"strings"
	entity2 "tesou.io/platform/foot-parent/foot-api/module/elem/pojo"
	"tesou.io/platform/foot-parent/foot-api/module/match/pojo"
	entity3 "tesou.io/platform/foot-parent/foot-api/module/odds/pojo"
	"tesou.io/platform/foot-parent/foot-core/module/elem/service"
	service2 "tesou.io/platform/foot-parent/foot-core/module/odds/service"
	"tesou.io/platform/foot-parent/foot-spider/module/win007"
	"tesou.io/platform/foot-parent/foot-spider/module/win007/vo"
)

type EuroLastProcesser struct {
	service.CompService
	service2.EuroLastService
	service2.EuroHisService
	//博彩公司对应的win007id
	CompWin007Ids      []string
	MatchLastList      []*pojo.MatchLast
	Win007idMatchidMap map[string]string
}

func GetEuroLastProcesser() *EuroLastProcesser {
	return &EuroLastProcesser{}
}

func (this *EuroLastProcesser) Startup() {
	this.Win007idMatchidMap = map[string]string{}

	newSpider := spider.NewSpider(this, "EuroLastProcesser")

	for _, v := range this.MatchLastList {
		i := v.Ext[win007.MODULE_FLAG]
		bytes, _ := json.Marshal(i)
		matchExt := new(pojo.MatchExt)
		json.Unmarshal(bytes, matchExt)

		win007_id := matchExt.Sid

		this.Win007idMatchidMap[win007_id] = v.Id

		url := strings.Replace(win007.WIN007_EUROODD_URL_PATTERN, "${matchId}", win007_id, 1)
		newSpider = newSpider.AddUrl(url, "html")
	}
	newSpider.SetDownloader(down.NewMWin007Downloader())
	newSpider = newSpider.AddPipeline(pipeline.NewPipelineConsole())
	newSpider.SetThreadnum(1).Run()
}

func (this *EuroLastProcesser) Process(p *page.Page) {
	request := p.GetRequest()
	if !p.IsSucc() {
		base.Log.Info("URL:,", request.Url, p.Errormsg())
		return
	}

	var hdata_str string
	p.GetHtmlParser().Find("script").Each(func(i int, selection *goquery.Selection) {
		text := selection.Text()
		if hdata_str == "" && strings.Contains(text, "var hData") {
			hdata_str = text
		} else {
			return
		}
	})
	if hdata_str == "" {
		return
	}

	// 获取script脚本中的，博彩公司信息
	hdata_str = strings.Replace(hdata_str, ";", "", 1)
	hdata_str = strings.Replace(hdata_str, "var hData = ", "", 1)
	base.Log.Info(hdata_str)

	this.hdata_process(request.Url, hdata_str)
}

func (this *EuroLastProcesser) hdata_process(url string, hdata_str string) {

	var hdata_list = make([]*vo.HData, 0)
	json.Unmarshal(([]byte)(hdata_str), &hdata_list)
	var regex_temp = regexp.MustCompile(`(\d+).htm`)
	win007Id := strings.Split(regex_temp.FindString(url), ".")[0]
	matchId := this.Win007idMatchidMap[win007Id]

	//入库中
	comp_list_slice := make([]interface{}, 0)
	last_slice := make([]interface{}, 0)
	last_update_slice := make([]interface{}, 0)
	for _, v := range hdata_list {
		comp := new(entity2.Comp)
		comp.Name = v.Cn
		comp_exists := this.CompService.Exist(comp)
		if !comp_exists {
			//comp.Id = bson.NewObjectId().Hex()
			comp.Id = strconv.Itoa(v.CId)
			comp_list_slice = append(comp_list_slice, comp)
		}

		//判断公司ID是否在配置的波菜公司队列中
		if len(this.CompWin007Ids) > 0 {
			var equal bool
			for _, id := range this.CompWin007Ids {
				if strings.EqualFold(id, strconv.Itoa(v.CId)) {
					equal = true
					break
				}
			}
			if !equal {
				continue
			}
		}

		last := new(entity3.EuroLast)
		last.MatchId = matchId
		last.CompId = comp.Id

		last.Sp3 = v.Hw
		last.Sp1 = v.So
		last.Sp0 = v.Gw
		last.Ep3 = v.Rh
		last.Ep1 = v.Rs
		last.Ep0 = v.Rg

		last_temp_id,last_exists := this.EuroLastService.Exist(last)
		if !last_exists {
			last_slice = append(last_slice, last)
		} else {
			last.Id = last_temp_id
			last_update_slice = append(last_update_slice, last)
		}
	}

	this.CompService.SaveList(comp_list_slice)
	//最后数据
	this.EuroLastService.SaveList(last_slice)
	this.EuroLastService.ModifyList(last_update_slice)
	//历史数据
	his_slice := make([]interface{}, 0)
	his_update_slice := make([]interface{}, 0)
	last_all_slice := append(last_slice,last_update_slice)
	for _,e := range last_all_slice {
		bytes, _ := json.Marshal(e)
		temp := new(entity3.EuroLast)
		json.Unmarshal(bytes, temp)
		if len(temp.MatchId) <= 0{
			continue
		}
		his := new(entity3.EuroHis)
		his.EuroLast = *temp

		his_temp_id,his_exists := this.EuroHisService.Exist(his)
		if !his_exists {
			his_slice = append(his_slice, his)
		} else {
			his.Id = his_temp_id
			his_update_slice = append(his_update_slice, his)
		}
	}
	this.EuroHisService.SaveList(his_slice)
	this.EuroHisService.ModifyList(his_update_slice)


}

func (this *EuroLastProcesser) Finish() {
	base.Log.Info("欧赔抓取解析完成 \r\n")

}
