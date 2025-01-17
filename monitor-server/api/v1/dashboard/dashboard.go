package dashboard

import (
	"encoding/json"
	"fmt"
	mid "github.com/WeBankPartners/open-monitor/monitor-server/middleware"
	"github.com/WeBankPartners/open-monitor/monitor-server/middleware/log"
	m "github.com/WeBankPartners/open-monitor/monitor-server/models"
	ds "github.com/WeBankPartners/open-monitor/monitor-server/services/datasource"
	"github.com/WeBankPartners/open-monitor/monitor-server/services/db"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// @Summary 页面通用接口 : 视图
// @Description 获取主视图，有主机、网络等
// @Produce  json
// @Param type query string true "页面类型，主机页面type=host"
// @Success 200
// @Router /api/v1/dashboard/main [get]
func MainDashboard(c *gin.Context) {
	dType := c.Query("type")
	if dType == "" {
		mid.ReturnParamEmptyError(c, "type")
		return
	}
	err, dashboard := db.GetDashboard(dType)
	if err != nil {
		mid.ReturnQueryTableError(c, "dashboard", err)
		return
	}
	var dashboardDto m.Dashboard
	if dashboard.SearchEnable {
		err, search := db.GetSearch(dashboard.SearchId)
		if err == nil {
			search.Enable = true
			dashboardDto.Search = search
		}
	}
	if dashboard.ButtonEnable {
		err, button := db.GetButton(dashboard.ButtonGroup)
		if err == nil {
			dashboardDto.Buttons = button
		}
	}
	if dashboard.MessageEnable {
		var message m.MessageModel
		message.Enable = true
		messageUrl := strings.Replace(dashboard.MessageUrl, "{group}", fmt.Sprintf("%d", dashboard.MessageGroup), -1)
		message.Url = messageUrl
		dashboardDto.Message = message
	}
	if dashboard.PanelsEnable {
		var panels m.PanelsModel
		panels.Enable = true
		panels.Type = dashboard.PanelsType
		panels.Url = fmt.Sprintf("/dashboard/panels?group=%d", dashboard.PanelsGroup)
		if dashboard.PanelsParam != "" {
			panels.Url = panels.Url + `&` + dashboard.PanelsParam
			if dashboard.SearchEnable == false && len(dashboardDto.Buttons) > 0 {
				defaultBV := dashboard.PanelsParam[strings.Index(dashboard.PanelsParam, "{")+1 : strings.Index(dashboard.PanelsParam, "}")]
				for _, v := range dashboardDto.Buttons {
					if v.Name == defaultBV {
						panels.Url = strings.Replace(panels.Url, fmt.Sprintf("{%s}", defaultBV), v.Options[0].OptionValue, -1)
					}
				}
			}
		}
		dashboardDto.Panels = panels
	}
	mid.ReturnSuccessData(c, dashboardDto)
}

// @Summary 页面通用接口 : 获取panels
// @Description 获取panels
// @Produce  json
// @Param group query int true "panels url 上自带该id"
// @Param endpoint query string true "需要在panels url上把{endpoint}替换"
// @Success 200
// @Router /api/v1/dashboard/panels [get]
func GetPanels(c *gin.Context) {
	group := c.Query("group")
	endpoint := c.Query("endpoint")
	if group == "" {
		mid.ReturnParamEmptyError(c, "group")
		return
	}
	groupId, err := strconv.Atoi(group)
	if err != nil {
		mid.ReturnParamTypeError(c, "group", "int")
		return
	}
	err, panels := db.GetPanels(groupId, endpoint)
	if err != nil {
		mid.ReturnQueryTableError(c, "panel", err)
		return
	}
	var panelsDto []*m.PanelModel
	for _, panel := range panels {
		//if panel.AutoDisplay > 0 && !endpointBusinessShow {
		//	continue
		//}
		if panel.AutoDisplay == 1 {
			continue
		}
		var panelDto m.PanelModel
		panelDto.Title = panel.Title
		if panel.ChartGroup < 0 {
			panelDto.Other = true
			panelsDto = append(panelsDto, &panelDto)
			continue
		}
		panelDto.Other = false
		err, charts := db.GetCharts(panel.ChartGroup, 0, 0)
		if err != nil {
			continue
		}
		tagsDto := m.TagsModel{Enable: false, Option: []*m.OptionModel{}}
		tagsValue := ""
		if panel.TagsEnable && endpoint != "" {
			var options []*m.OptionModel
			tagsDto.Enable = true
			tagsDto.Url = fmt.Sprintf(`%s?panel_id=%d&endpoint=%s&tag=`, panel.TagsUrl, panel.Id, endpoint)
			err, options = db.GetTags(endpoint, panel.TagsKey, charts[0].Metric)
			if err == nil {
				tagsDto.Option = options
				if len(options) > 0 {
					tagsValue = options[0].OptionText
				}
			}
		}
		panelDto.Tags = tagsDto
		var chartsDto []*m.ChartModel
		for _, chart := range charts {
			chartDto := m.ChartModel{Id: chart.Id, Col: chart.Col}
			chartDto.Url = `/dashboard/chart`
			chartDto.Endpoint = []string{endpoint}
			metricList := strings.Split(chart.Metric, "^")
			if panel.TagsEnable && tagsValue != "" {
				var newMetricList []string
				for _, m := range metricList {
					newMetric := m + `/` + panel.TagsKey + `=` + tagsValue
					newMetricList = append(newMetricList, newMetric)
				}
				chartDto.Metric = newMetricList
			} else {
				chartDto.Metric = metricList
			}
			chartsDto = append(chartsDto, &chartDto)
		}
		panelDto.Charts = chartsDto
		panelsDto = append(panelsDto, &panelDto)
	}
	businessPanel, matchErr := db.MatchServicePanel(endpoint)
	//err,businessPanel := fetchBusinessPanel(endpoint)
	if matchErr != nil {
		log.Logger.Error("Fetch business panel fail", log.Error(matchErr))
	}
	if len(businessPanel.Charts) > 0 {
		panelsDto = append(panelsDto, &businessPanel)
	}
	mid.ReturnSuccessData(c, panelsDto)
}

func fetchBusinessPanel(endpoint string) (err error, result m.PanelModel) {
	result.Tags = m.TagsModel{Enable: false, Option: []*m.OptionModel{}}
	var businessList m.BusinessUpdateDto
	realEndpoint := endpoint
	endpointObj := m.EndpointTable{Guid: endpoint}
	db.GetEndpoint(&endpointObj)
	if endpointObj.ExportType == "host" {
		err, businessList = db.GetBusinessListNew(endpointObj.Id, "")
	} else {
		err, businessList = db.GetBusinessListNew(0, endpoint)
		realEndpoint = db.GetBusinessRealEndpoint(endpoint)
	}
	if err != nil || len(businessList.PathList) == 0 {
		return err, result
	}
	chartTable, panelTable := db.GetBusinessPanelChart()
	if len(panelTable) == 0 || len(chartTable) == 0 {
		return err, result
	}
	result.Title = panelTable[0].Title
	//var promMetricKeys []string
	for _, path := range businessList.PathList {
		for _, rule := range path.Rules {
			for _, metricConfig := range rule.MetricConfig {
				//promMetricKeys = append(promMetricKeys, metricConfig.Metric)
				tmpChartObj := m.ChartModel{Id: chartTable[0].Id, Endpoint: []string{realEndpoint}, Url: chartTable[0].Url}
				tmpChartObj.Title = metricConfig.Title
				tmpChartObj.Metric = []string{fmt.Sprintf("%s/path=%s,key=%s", chartTable[0].Metric, path.Path, metricConfig.Metric)}
				result.Charts = append(result.Charts, &tmpChartObj)
			}
		}
		for _, custom := range path.CustomMetrics {
			tmpChartObj := m.ChartModel{Id: chartTable[0].Id, Endpoint: []string{realEndpoint}, Url: chartTable[0].Url}
			tmpChartObj.Title = custom.Metric
			tmpChartObj.Metric = []string{fmt.Sprintf("%s/path=%s,key=%s", chartTable[0].Metric, path.Path, custom.Metric)}
			result.Charts = append(result.Charts, &tmpChartObj)
		}
	}
	//_,extendMetric := db.GetBusinessPromMetric(promMetricKeys)
	//for _,v := range extendMetric {
	//	tmpChartObj := m.ChartModel{Id: chartTable[0].Id, Endpoint: []string{realEndpoint}, Url: chartTable[0].Url}
	//	tmpChartObj.Metric = []string{v.Metric}
	//	tmpChartObj.Title = v.Metric
	//	result.Charts = append(result.Charts, &tmpChartObj)
	//}
	return err, result
}

func UpdateChartsTitle(c *gin.Context) {
	var param m.UpdateChartTitleParam
	if err := c.ShouldBindJSON(&param); err == nil {
		if param.ChartId > 0 {
			err = db.UpdateChartTitle(param)
		} else {
			err = db.UpdateServiceMetricTitle(param)
		}
		if err != nil {
			mid.ReturnUpdateTableError(c, "chart", err)
		} else {
			mid.ReturnSuccess(c)
		}
	} else {
		mid.ReturnValidateError(c, err.Error())
	}
}

// @Summary 页面通用接口 : 根据tag获取charts组
// @Description 根据tag获取charts组
// @Produce  json
// @Param panel_id query int true "url上自带该id"
// @Param endpoint query string true "url上自带该endpoint"
// @Param tag query string true "tag button里面的option_value"
// @Success 200
// @Router /api/v1/dashboard/tags [get]
func GetTags(c *gin.Context) {
	panelIdStr := c.Query("panel_id")
	endpoint := c.Query("endpoint")
	tag := c.Query("tag")
	if tag == "" {
		mid.ReturnParamEmptyError(c, "tag")
		return
	}
	panelId, err := strconv.Atoi(panelIdStr)
	if err != nil {
		mid.ReturnParamTypeError(c, "panel_id", "int")
		return
	}
	err, charts := db.GetCharts(0, 0, panelId)
	var chartsDto []*m.ChartModel
	for _, chart := range charts {
		chartDto := m.ChartModel{Id: chart.Id, Col: chart.Col}
		chartDto.Url = `/dashboard/chart`
		if endpoint != "" {
			chartDto.Endpoint = []string{endpoint}
		}
		metricList := strings.Split(chart.Metric, "^")
		var newMetricList []string
		for _, m := range metricList {
			newMetric := m + `/` + tag
			newMetricList = append(newMetricList, newMetric)
		}
		chartDto.Metric = newMetricList
		chartsDto = append(chartsDto, &chartDto)
	}
	mid.ReturnSuccessData(c, chartsDto)
}

// @Summary 页面通用接口 : 获取chart数据
// @Description 获取chart数据
// @Produce  json
// @Param id query int true "panel里的chart id"
// @Param endpoint query []string true "endpoint数组, ['88B525B4-43E8-4A7A-8E11-0E664B5CB8D0']"
// @Param metric query []string true "metric数组, ['cpmid.busy']"
// @Param start query string true "开始时间"
// @Param end query string false "结束时间"
// @Param aggregate query string false "聚合类型 枚举 min max avg p95 none"
// @Success 200
// @Router /api/v1/dashboard/chart [get]
func GetChartOld(c *gin.Context) {
	paramId, err := strconv.Atoi(c.Query("id"))
	if err != nil || paramId <= 0 {
		mid.ReturnParamTypeError(c, "id", "int")
		return
	}
	err, charts := db.GetCharts(0, paramId, 0)
	if err != nil || len(charts) <= 0 {
		mid.ReturnQueryTableError(c, "chart", err)
		return
	}
	chart := *charts[0]
	var eOption m.EChartOption
	var query m.QueryMonitorData
	eOption.Id = paramId
	eOption.Title = chart.Title
	if chart.Endpoint == "" {
		query.Endpoint = c.QueryArray("endpoint[]")
	} else {
		query.Endpoint = strings.Split(chart.Endpoint, "^")
	}
	if len(query.Endpoint) <= 0 {
		mid.ReturnValidateError(c, "Parameter \"endpoint\" validation failed")
		return
	}
	query.Metric = c.QueryArray("metric[]")
	if chart.Metric != "" && len(query.Metric) > 0 {
		if !strings.Contains(query.Metric[0], "/") {
			query.Metric = strings.Split(chart.Metric, "^")
		}
	}
	paramTime := c.Query("time")
	paramStart := c.Query("start")
	paramEnd := c.Query("end")
	if paramTime != "" && paramStart == "" {
		paramStart = paramTime
	}
	start, err := strconv.ParseInt(paramStart, 10, 64)
	if err != nil {
		mid.ReturnParamTypeError(c, "start", "int")
		return
	} else {
		if start < 0 {
			start = time.Now().Unix() + start
		}
		query.Start = start
	}
	query.End = time.Now().Unix()
	if paramEnd != "" {
		end, err := strconv.ParseInt(paramEnd, 10, 64)
		if err == nil && end <= query.End {
			query.End = end
		}
	}
	err, query.PromQ = db.GetPromMetric(query.Endpoint, query.Metric[0])
	if err != nil {
		mid.ReturnQueryTableError(c, "prom_ql", err)
		return
	}
	query.Legend = chart.Legend
	log.Logger.Debug("Query param", log.StringList("endpoint", query.Endpoint), log.StringList("metric", query.Metric), log.Int64("start", query.Start), log.Int64("end", query.End), log.String("promQl", query.PromQ))
	serials := ds.PrometheusData(&query)
	agg := db.CheckAggregate(query.Start, query.End, query.Endpoint[0], 0, len(serials))
	for _, s := range serials {
		if strings.Contains(s.Name, "$metric") {
			s.Name = strings.Replace(s.Name, "$metric", query.Metric[0], -1)
		}
		eOption.Legend = append(eOption.Legend, s.Name)
		if agg > 1 {
			aggType := chart.AggType
			if c.Query("agg") != "" {
				aggType = c.Query("agg")
			}
			if aggType != "none" && aggType != "" {
				s.Data = db.Aggregate(s.Data, agg, aggType)
			}
		}
	}
	eOption.Xaxis = make(map[string]interface{})
	eOption.Yaxis = m.YaxisModel{Unit: chart.Unit}
	if len(serials) > 0 {
		eOption.Series = serials
	} else {
		eOption.Series = []*m.SerialModel{}
	}
	mid.ReturnSuccessData(c, eOption)
}

func GetChart(c *gin.Context) {
	requestBody, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		mid.ReturnBodyError(c, err)
		return
	}
	var paramConfig []m.ChartConfigObj
	err = json.Unmarshal(requestBody, &paramConfig)
	if err != nil {
		mid.ReturnRequestJsonError(c, err)
		return
	}
	if len(paramConfig) == 0 {
		mid.ReturnParamEmptyError(c, "")
		return
	}
	var eOption m.EChartOption
	var query m.QueryMonitorData
	var compareLegend string
	var sameEndpoint bool
	var aggType string
	var subStartSecond, subEndSecond int64
	// validate config time
	if paramConfig[0].CompareFirstStart != "" && paramConfig[0].CompareFirstEnd != "" {
		st, err := time.Parse(m.DateFormatWithZone, fmt.Sprintf("%s 00:00:00 "+m.DefaultLocalTimeZone, paramConfig[0].CompareFirstStart))
		if err != nil {
			mid.ReturnParamTypeError(c, "compare_first_start", "2006-01-02")
			return
		}
		et, err := time.Parse(m.DateFormatWithZone, fmt.Sprintf("%s 23:59:59 "+m.DefaultLocalTimeZone, paramConfig[0].CompareFirstEnd))
		if err != nil {
			mid.ReturnParamTypeError(c, "compare_first_end", "2006-01-02")
			return
		}
		if paramConfig[0].Start != "" && paramConfig[0].End != "" {
			query.Start, _ = strconv.ParseInt(paramConfig[0].Start, 10, 64)
			query.End, _ = strconv.ParseInt(paramConfig[0].End, 10, 64)
			subStartSecond = query.Start - st.Unix()
			subEndSecond = query.End - st.Unix()
		} else {
			query.Start = st.Unix()
			query.End = et.Unix()
		}
		compareLegend = fmt.Sprintf("%s_%s", paramConfig[0].CompareFirstStart, paramConfig[0].CompareFirstEnd)
	} else {
		if paramConfig[0].Time != "" && paramConfig[0].Start == "" {
			paramConfig[0].Start = paramConfig[0].Time
		}
		start, err := strconv.ParseInt(paramConfig[0].Start, 10, 64)
		if err != nil {
			mid.ReturnParamTypeError(c, "start", "intString")
			return
		} else {
			if start < 0 {
				start = time.Now().Unix() + start
			}
			query.Start = start
		}
		query.End = time.Now().Unix()
		if paramConfig[0].End != "" {
			end, err := strconv.ParseInt(paramConfig[0].End, 10, 64)
			if err == nil && end <= query.End {
				query.End = end
			}
		}
	}
	// custom or from mysql
	var querys []m.QueryMonitorData
	step := 10
	var firstEndpoint, unit, clusterAddress string
	var compareSubTime int64
	var compareSecondLegend string
	if paramConfig[0].Id > 0 {
		// Handle config dashboard
		sameEndpoint = true
		recordMap := make(map[string]bool)
		firstEndpointObj := m.EndpointTable{Guid: paramConfig[0].Endpoint}
		db.GetEndpoint(&firstEndpointObj)
		step = firstEndpointObj.Step
		clusterAddress = db.GetClusterAddress(firstEndpointObj.Cluster)
		// one endpoint -> metrics
		for _, tmpParamConfig := range paramConfig {
			err, charts := db.GetCharts(0, tmpParamConfig.Id, 0)
			if err != nil || len(charts) <= 0 {
				mid.ReturnQueryTableError(c, "chart", err)
				return
			}
			chart := *charts[0]
			tmpIndex := fmt.Sprintf("%d^%s", tmpParamConfig.Id, tmpParamConfig.Endpoint)
			if _, b := recordMap[tmpIndex]; b {
				if chart.Metric != "db_monitor_count" {
					continue
				}
			}
			recordMap[tmpIndex] = true
			aggType = chart.AggType
			eOption.Id = chart.Id
			eOption.Title = chart.Title
			if tmpParamConfig.Title != "" {
				eOption.Title = tmpParamConfig.Title
			}
			unit = chart.Unit
			if tmpParamConfig.Endpoint == "" {
				mid.ReturnParamEmptyError(c, "endpoint")
				return
			}
			if strings.Contains(tmpParamConfig.Metric, "/") {
				chart.Metric = tmpParamConfig.Metric
			}
			if chart.Metric == "app.metric" {
				chart.Metric = tmpParamConfig.Metric
			}
			for _, v := range strings.Split(chart.Metric, "^") {
				var tmpPromQl string
				if chart.Metric == "db_monitor_count" && tmpParamConfig.Metric != "db_monitor_count" {
					err, tmpPromQl = db.GetDbPromMetric(tmpParamConfig.Endpoint, tmpParamConfig.Metric, chart.Legend)
				} else {
					err, tmpPromQl = db.GetPromMetric([]string{tmpParamConfig.Endpoint}, v)
				}
				if err != nil {
					log.Logger.Error("Get prometheus metric failed", log.Error(err))
					continue
				}
				tmpLegend := chart.Legend
				if len(paramConfig) > 1 && strings.Contains(chart.Legend, "metric") {
					tmpLegend = "$custom"
				}
				querys = append(querys, m.QueryMonitorData{Start: query.Start, End: query.End, PromQ: tmpPromQl, Legend: tmpLegend, Metric: []string{v}, Endpoint: []string{tmpParamConfig.Endpoint}, CompareLegend: compareLegend, SameEndpoint: sameEndpoint, Step: step, Cluster: clusterAddress})
				if paramConfig[0].CompareSecondStart != "" && paramConfig[0].CompareSecondEnd != "" {
					st, sErr := time.Parse(m.DateFormatWithZone, fmt.Sprintf("%s 00:00:00 "+m.DefaultLocalTimeZone, paramConfig[0].CompareSecondStart))
					et, eErr := time.Parse(m.DateFormatWithZone, fmt.Sprintf("%s 23:59:59 "+m.DefaultLocalTimeZone, paramConfig[0].CompareSecondEnd))
					stTimestamp := st.Unix()
					etTimestamp := et.Unix()
					if sErr == nil && eErr == nil {
						if subStartSecond > 0 && subEndSecond > 0 {
							stTimestamp = st.Unix() + subStartSecond
							etTimestamp = st.Unix() + subEndSecond
						} else {
							if (et.Unix() - st.Unix()) != (query.End - query.Start) {
								etTimestamp = stTimestamp + (query.End - query.Start)
							}
						}
						compareSubTime = stTimestamp - query.Start
						compareSecondLegend = fmt.Sprintf("%s_%s", paramConfig[0].CompareSecondStart, paramConfig[0].CompareSecondEnd)
						querys = append(querys, m.QueryMonitorData{Start: stTimestamp, End: etTimestamp, PromQ: tmpPromQl, Legend: tmpLegend, Metric: []string{v}, Endpoint: []string{tmpParamConfig.Endpoint}, CompareLegend: compareSecondLegend, SameEndpoint: sameEndpoint, Step: step, Cluster: clusterAddress})
					}
				}
			}
		}
	} else {
		// Handle custom dashboard
		var customLegend, tmpEndpointParam, tmpMetricParam string
		var diffEndpoint, diffMetric bool
		for i, v := range paramConfig {
			if v.PromQl == "" {
				_, tmpPromQL := db.GetPromMetric([]string{v.Endpoint}, v.Metric)
				if tmpPromQL == "" {
					continue
				} else {
					paramConfig[i].PromQl = tmpPromQL
				}
			}
			if i == 0 {
				tmpEndpointParam = v.Endpoint
				tmpMetricParam = v.Metric
			} else {
				if tmpEndpointParam != v.Endpoint {
					diffEndpoint = true
				}
				if tmpMetricParam != v.Metric {
					diffMetric = true
				}
			}
		}
		if diffEndpoint && !diffMetric {
			customLegend = "$custom_endpoint"
		}
		if !diffEndpoint && diffMetric {
			customLegend = "$custom_metric"
		}
		if diffEndpoint == diffMetric {
			customLegend = "$custom"
		}
		for _, v := range paramConfig {
			if v.PromQl == "" {
				continue
			}
			endpointObj := m.EndpointTable{Guid: v.Endpoint}
			if v.Endpoint != "" {
				db.GetEndpoint(&endpointObj)
				clusterAddress = db.GetClusterAddress(endpointObj.Cluster)
			}
			if strings.Contains(v.PromQl, "$address") {
				if v.Endpoint == "" {
					continue
				}
				if endpointObj.Address == "" {
					continue
				}
				if endpointObj.AddressAgent != "" {
					v.PromQl = strings.Replace(v.PromQl, "$address", endpointObj.AddressAgent, -1)
				} else {
					v.PromQl = strings.Replace(v.PromQl, "$address", endpointObj.Address, -1)
				}
				step = endpointObj.Step
			}
			if strings.Contains(v.PromQl, "$guid") {
				v.PromQl = strings.Replace(v.PromQl, "$guid", v.Endpoint, -1)
			}
			if strings.Contains(v.PromQl, "$pod") {
				v.PromQl = strings.Replace(v.PromQl, "$pod", endpointObj.Name[len(endpointObj.ExportVersion)+1:], -1)
			}
			if strings.Contains(v.PromQl, "$k8s_namespace") {
				v.PromQl = strings.Replace(v.PromQl, "$k8s_namespace", endpointObj.ExportVersion, -1)
			}
			if strings.Contains(v.PromQl, "$k8s_cluster") {
				v.PromQl = strings.Replace(v.PromQl, "$k8s_cluster", endpointObj.OsType, -1)
			}
			if strings.Contains(v.PromQl, "$") {
				re, _ := regexp.Compile("=\"[\\$]+[^\"]+\"")
				fetchTag := re.FindAll([]byte(v.PromQl), -1)
				for _, vv := range fetchTag {
					v.PromQl = strings.Replace(v.PromQl, string(vv), "=~\".*\"", -1)
				}
			}
			querys = append(querys, m.QueryMonitorData{Start: query.Start, End: query.End, PromQ: v.PromQl, Legend: customLegend, Metric: []string{v.Metric}, Endpoint: []string{v.Endpoint}, Step: step, Cluster: clusterAddress})
		}
	}
	if len(querys) == 0 {
		mid.ReturnHandleError(c, "Query list is empty", nil)
		return
	}
	var serials []*m.SerialModel
	archiveQueryFlag := false
	if query.Start < (time.Now().Unix()-m.Config().ArchiveMysql.LocalStorageMaxDay*86400) && db.ArchiveEnable {
		archiveQueryFlag = true
	}
	appendDataFlag := false
	for _, v := range querys {
		log.Logger.Debug("Query param", log.StringList("endpoint", v.Endpoint), log.StringList("metric", v.Metric), log.Int64("start", v.Start), log.Int64("end", v.End), log.String("promQl", v.PromQ))
		if !archiveQueryFlag {
			tmpSerials := ds.PrometheusData(&v)
			if len(tmpSerials) > 0 {
				if len(tmpSerials[0].Data) > 0 {
					tmpSerialEnd := int64(tmpSerials[0].Data[0][0]) / 1000
					if tmpSerialEnd > (query.Start + 120) {
						_, _, tmpArchiveSerials := db.GetArchiveData(&m.QueryMonitorData{Start: v.Start, End: tmpSerialEnd, Endpoint: v.Endpoint, Metric: v.Metric, Legend: v.Legend, CompareLegend: v.CompareLegend, SameEndpoint: v.SameEndpoint}, paramConfig[0].Aggregate)
						for _, tmpSerial := range tmpArchiveSerials {
							if len(tmpSerial.Data) > 0 {
								appendDataFlag = true
								for si, vv := range tmpSerials {
									if tmpSerial.Name == vv.Name {
										tmpSerials[si].Data = append(tmpSerial.Data, vv.Data...)
									}
								}
							}
						}
					}
				}
			} else {
				err, step, tmpSerials = db.GetArchiveData(&m.QueryMonitorData{Start: v.Start, End: v.End, Endpoint: v.Endpoint, Metric: v.Metric, Legend: v.Legend, CompareLegend: v.CompareLegend, SameEndpoint: v.SameEndpoint}, paramConfig[0].Aggregate)
				if err != nil {
					log.Logger.Error("Prometheus no data,try to get archive data error", log.Error(err))
				}
			}
			for _, vv := range tmpSerials {
				serials = append(serials, vv)
			}
		} else {
			err, tmpStep, tmpSerials := db.GetArchiveData(&v, paramConfig[0].Aggregate)
			if err != nil {
				mid.ReturnQueryTableError(c, "prometheus_archive", err)
				return
			}
			for _, vv := range tmpSerials {
				serials = append(serials, vv)
			}
			step = tmpStep
		}
	}
	// agg
	agg := 0
	if !appendDataFlag {
		agg = db.CheckAggregate(query.Start, query.End, firstEndpoint, step, len(serials))
	}
	//var firstSerialTime float64
	eOption.Series = []*m.SerialModel{}
	processDisplayMap := db.GetProcessDisplayMap(paramConfig[0].Endpoint)
	for i, s := range serials {
		if strings.Contains(s.Name, "$metric") {
			queryIndex := i
			if i >= len(querys) {
				queryIndex = len(querys) - 1
			}
			s.Name = strings.Replace(s.Name, "$metric", querys[queryIndex].Metric[0], -1)
		}
		if processName, b := processDisplayMap[s.Name]; b {
			s.Name = processName
		}
		eOption.Legend = append(eOption.Legend, s.Name)
		if eOption.Title == "${auto}" {
			eOption.Title = s.Name[:strings.Index(s.Name, "{")]
		}
		if agg > 1 && len(s.Data) > 300 {
			if paramConfig[0].Aggregate != "" {
				aggType = paramConfig[0].Aggregate
			}
			if aggType == "" {
				aggType = "avg"
			}
			if aggType != "none" {
				s.Data = db.Aggregate(s.Data, agg, aggType)
			}
		}
		if compareSubTime > 0 {
			if strings.Contains(s.Name, compareSecondLegend) {
				s.Data = db.CompareSubData(s.Data, float64(compareSubTime)*1000)
			}
		}
		_, tmpJsonMarshalErr := json.Marshal(s)
		if tmpJsonMarshalErr == nil {
			eOption.Series = append(eOption.Series, s)
		}
	}
	eOption.Xaxis = make(map[string]interface{})
	eOption.Yaxis = m.YaxisModel{Unit: unit}
	mid.ReturnSuccessData(c, eOption)
}

func GetPieChart(c *gin.Context) {
	requestBody, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		mid.ReturnBodyError(c, err)
		return
	}
	var paramConfig m.ChartConfigObj
	err = json.Unmarshal(requestBody, &paramConfig)
	if err != nil {
		mid.ReturnRequestJsonError(c, err)
		return
	}
	var query m.QueryMonitorData
	if paramConfig.Endpoint == "" || paramConfig.Metric == "" {
		mid.ReturnParamEmptyError(c, "endpoint and metric")
		return
	}
	endpointObj := m.EndpointTable{Guid: paramConfig.Endpoint}
	db.GetEndpoint(&endpointObj)
	if endpointObj.Id <= 0 {
		mid.ReturnFetchDataError(c, "endpoint", "guid", paramConfig.Endpoint)
		return
	}
	query.Start = time.Now().Unix() - 120
	query.End = time.Now().Unix()
	// fetch promQL
	if paramConfig.PromQl == "" {
		_, tmpPromQL := db.GetPromMetric([]string{paramConfig.Endpoint}, paramConfig.Metric)
		if tmpPromQL == "" {
			mid.ReturnFetchDataError(c, "prom_ql", "endpoint,metric", paramConfig.Endpoint+","+paramConfig.Metric)
			return
		} else {
			paramConfig.PromQl = tmpPromQL
		}
	}
	if strings.Contains(paramConfig.PromQl, "$address") {
		tmpAddress := endpointObj.Address
		if endpointObj.AddressAgent != "" {
			tmpAddress = endpointObj.AddressAgent
		}
		paramConfig.PromQl = strings.Replace(paramConfig.PromQl, "$address", tmpAddress, -1)
	}
	queryResult := m.QueryMonitorData{Start: query.Start, End: query.End, PromQ: paramConfig.PromQl, Metric: []string{paramConfig.Metric}, Endpoint: []string{paramConfig.Endpoint}, ChartType: "pie"}
	ds.PrometheusData(&queryResult)
	mid.ReturnSuccessData(c, queryResult.PieData)
}

// @Summary 主页面接口 : 模糊搜索
// @Description 模糊搜索
// @Produce  json
// @Param search query string true "放弃search_col,直接把用户输入拼到url后面请求"
// @Param limit query string false "数量限制"
// @Success 200
// @Router /api/v1/dashboard/search [get]
func MainSearch(c *gin.Context) {
	endpoint := c.Query("search")
	//limit := c.Query("limit")
	if endpoint == "" {
		mid.ReturnParamEmptyError(c, "search")
		return
	}
	tmpFlag := false
	if strings.Contains(endpoint, `:`) {
		endpoint = strings.Split(endpoint, `:`)[1]
		tmpFlag = true
	}
	err, result := db.SearchHost(endpoint)
	if err != nil {
		mid.ReturnQueryTableError(c, "endpoint", err)
		return
	}
	for _, v := range result {
		v.OptionTypeName = v.OptionType
	}
	sysResult := db.SearchRecursivePanel(endpoint)
	for _, v := range sysResult {
		result = append(result, v)
	}
	if tmpFlag {
		var tmpResult []*m.OptionModel
		for _, v := range result {
			if v.OptionText == c.Query("search") {
				tmpResult = append(tmpResult, v)
				break
			}
		}
		if len(tmpResult) > 0 {
			result = tmpResult
		}
	}
	var sortOptionList m.OptionModelSortList
	sortOptionList = append(sortOptionList, result...)
	sort.Sort(sortOptionList)
	mid.ReturnSuccessData(c, sortOptionList)
}

func GetPromMetric(c *gin.Context) {
	metricType := c.Query("type")
	err, data := db.GetPromMetricTable(metricType)
	if err != nil {
		mid.ReturnQueryTableError(c, "prom_metric", err)
		return
	}
	mid.ReturnSuccessData(c, data)
}

func UpdatePanelChartMetric(c *gin.Context) {
	var param []m.PromMetricUpdateParam
	if err := c.ShouldBindJSON(&param); err == nil {
		if len(param) == 0 {
			mid.ReturnParamEmptyError(c, "")
			return
		}
		err := db.UpdatePanelChartMetric(param)
		if err != nil {
			mid.ReturnUpdateTableError(c, "prom_metric", err)
			return
		}
		mid.ReturnSuccess(c)
	} else {
		mid.ReturnValidateError(c, err.Error())
	}
}

func UpdatePromMetric(c *gin.Context) {
	var param []*m.PromMetricTable
	if err := c.ShouldBindJSON(&param); err == nil {
		if len(param) == 0 {
			mid.ReturnParamEmptyError(c, "")
			return
		}
		err := db.UpdatePromMetric(param)
		if err != nil {
			mid.ReturnUpdateTableError(c, "prom_metric", err)
			return
		}
		mid.ReturnSuccess(c)
	} else {
		mid.ReturnValidateError(c, err.Error())
	}
}

func GetEndpointMetric(c *gin.Context) {
	var param m.GetEndpointMetricParam
	if err:=c.ShouldBindJSON(&param);err!=nil {
		mid.ReturnValidateError(c, err.Error())
		return
	}
	var err error
	var data []*m.OptionModel
	if param.ServiceGroup != "" {
		err, data = db.GetServiceGroupPromMetric(param.ServiceGroup, param.Workspace, param.MonitorType)
	}else {
		err, data = db.GetEndpointMetric(param.Guid, param.MonitorType)
	}
	if err != nil {
		mid.ReturnHandleError(c, "Get endpoint metric failed", err)
		return
	}
	mid.ReturnSuccessData(c, data)
}

func GetChartsByEndpoint(c *gin.Context) {
	// Validate ip and metric
	ip := c.Query("ip")
	metric := c.Query("metric")
	if ip == "" || metric == "" {
		mid.ReturnParamEmptyError(c, "ip or metric")
		return
	}
	endpointObj := m.EndpointTable{Ip: ip, ExportType: "host"}
	db.GetEndpoint(&endpointObj)
	if endpointObj.Id <= 0 {
		mid.ReturnFetchDataError(c, "endpoint", "ip", ip)
		return
	}
	err, promQL := db.GetPromMetric([]string{endpointObj.Guid}, metric)
	if err != nil || promQL == "" {
		mid.ReturnFetchDataError(c, "prom_ql", "endpoint,metric", endpointObj.Guid+","+metric)
		return
	}
	var eOption m.EChartOption
	var query m.QueryMonitorData
	query.Endpoint = []string{endpointObj.Guid}
	query.Metric = []string{metric}
	query.PromQ = promQL
	query.Legend = "$metric"
	// Validate time start end
	paramTime := c.Query("time")
	paramStart := c.Query("start")
	paramEnd := c.Query("end")
	if paramTime != "" && paramStart == "" {
		paramStart = paramTime
	}
	start, err := strconv.ParseInt(paramStart, 10, 64)
	if err != nil {
		mid.ReturnParamTypeError(c, "start", "intString")
		return
	} else {
		if start < 0 {
			start = time.Now().Unix() + start
		}
		query.Start = start
	}
	query.End = time.Now().Unix()
	if paramEnd != "" {
		end, err := strconv.ParseInt(paramEnd, 10, 64)
		if err == nil && end <= query.End {
			query.End = end
		}
	}
	// Query data
	log.Logger.Debug("Query param", log.StringList("endpoint", query.Endpoint), log.StringList("metric", query.Metric), log.Int64("start", query.Start), log.Int64("end", query.End), log.String("promQl", query.PromQ))
	serials := ds.PrometheusData(&query)
	for _, s := range serials {
		if strings.Contains(s.Name, "$metric") {
			s.Name = strings.Replace(s.Name, "$metric", metric, -1)
		}
		eOption.Legend = append(eOption.Legend, s.Name)
	}
	eOption.Xaxis = make(map[string]interface{})
	var unit string
	if strings.Contains(metric, "percent") {
		unit = "%"
	}
	eOption.Yaxis = m.YaxisModel{Unit: unit}
	if len(serials) > 0 {
		eOption.Series = serials
	} else {
		eOption.Series = []*m.SerialModel{}
	}
	mid.ReturnSuccessData(c, eOption)
}

func GetMainPage(c *gin.Context) {
	var roleList []string
	var user string
	token := mid.GetCoreToken(c)
	roleList = token.Roles
	user = token.User
	if user == "" || len(roleList) == 0 {
		user = mid.GetOperateUser(c)
		_, userRoleList := db.GetUserRole(user)
		for _, v := range userRoleList {
			roleList = append(roleList, v.Name)
		}
	}
	err, result := db.GetMainCustomDashboard(roleList)
	if err != nil {
		log.Logger.Error("Get main page failed", log.Error(err))
	}
	mid.ReturnSuccessData(c, result)
}

func ListMainPageRole(c *gin.Context) {
	var roleList []string
	var user string
	token := mid.GetCoreToken(c)
	roleList = token.Roles
	user = token.User
	if user == "" || len(roleList) == 0 {
		user = mid.GetOperateUser(c)
		_, userRoleList := db.GetUserRole(user)
		for _, v := range userRoleList {
			roleList = append(roleList, v.Name)
		}
	}
	err, result := db.ListMainPageRole(user, roleList)
	if err != nil {
		mid.ReturnHandleError(c, err.Error(), err)
	} else {
		if result == nil {
			result = []*m.MainPageRoleQuery{}
		}
		mid.ReturnSuccessData(c, result)
	}
}

func UpdateMainPage(c *gin.Context) {
	var param []m.MainPageRoleQuery
	if err := c.ShouldBindJSON(&param); err == nil {
		err := db.UpdateMainPageRole(param)
		if err != nil {
			mid.ReturnHandleError(c, err.Error(), err)
			return
		}
		mid.ReturnSuccess(c)
	} else {
		mid.ReturnValidateError(c, err.Error())
	}
}

func GetEndpointsByIp(c *gin.Context) {
	ipList := c.QueryArray("ip")
	if len(ipList) <= 0 {
		mid.ReturnParamEmptyError(c, "ip")
		return
	}
	err, endpoints := db.GetEndpointsByIp(ipList, "host")
	if err != nil {
		mid.ReturnQueryTableError(c, "endpoint", err)
		return
	}
	mid.ReturnSuccessData(c, endpoints)
}

func DisplayWatermark(c *gin.Context) {
	result := m.DisplayDemoFlagDto{Display: true}
	isDisplay := strings.ToLower(os.Getenv("DEMO_FLAG"))
	if isDisplay == "n" || isDisplay == "no" || isDisplay == "false" {
		result.Display = false
	}
	mid.ReturnSuccessData(c, result)
}

func GetDashboardPanelList(c *gin.Context) {
	endpointType := c.Query("type")
	metric := c.Query("metric")
	if endpointType == "" || metric == "" {
		mid.ReturnValidateError(c, "Param type and metric can not empty")
		return
	}
	result := db.GetDashboardPanelList(endpointType, metric)
	mid.ReturnSuccessData(c, result)
}
