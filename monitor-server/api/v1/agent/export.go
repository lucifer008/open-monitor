package agent

import (
	"encoding/json"
	"fmt"
	mid "github.com/WeBankPartners/open-monitor/monitor-server/middleware"
	"github.com/WeBankPartners/open-monitor/monitor-server/middleware/log"
	m "github.com/WeBankPartners/open-monitor/monitor-server/models"
	"github.com/WeBankPartners/open-monitor/monitor-server/services/db"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

type resultObj struct {
	ResultCode  string  `json:"resultCode"`
	ResultMessage  string  `json:"resultMessage"`
	Results  resultOutput  `json:"results"`
}

type resultOutput struct {
	Outputs  []resultOutputObj  `json:"outputs"`
}

type resultOutputObj struct {
	CallbackParameter  string  `json:"callbackParameter"`
	Guid  string  `json:"guid"`
	MonitorKey  string  `json:"monitor_key"`
	ErrorCode  string  `json:"errorCode"`
	ErrorMessage  string  `json:"errorMessage"`
	ErrorDetail  string  `json:"errorDetail,omitempty"`
}

type requestObj struct {
	RequestId  string  	`json:"requestId"`
	Inputs  []endpointRequestObj  `json:"inputs"`
}

type endpointRequestObj struct {
	Guid  string  `json:"guid"`
	CallbackParameter  string  `json:"callbackParameter"`
	HostIp  string  `json:"host_ip"`
	InstanceIp  string  `json:"instance_ip"`
	Group  string  `json:"group"`
	Port  string  `json:"port"`
	Instance  string  `json:"instance"`
	User  string  `json:"user"`
	Password  string  `json:"password"`
	JavaType  string  `json:"java_type"`
	PasswordGuid  string  `json:"password_guid"`
	PasswordSeed  string  `json:"password_seed"`
	AppLogPaths   string  `json:"app_log_paths"`
	Step  string  `json:"step"`
	Url   string  `json:"url"`
	Method  string  `json:"method"`
	Pod  string  `json:"pod"`
	KubernetesCluster  string  `json:"kubernetes_cluster"`
}

func ExportAgentNew(c *gin.Context)  {
	agentType := c.Param("name")
	action := "register"
	if strings.Contains(c.Request.URL.String(), "deregister") {
		action = "deregister"
	}
	var resultCode,resultMessage string
	resultCode = "0"
	resultData := resultOutput{}
	defer func() {
		log.Logger.Info("Plugin result", log.JsonObj("result", resultData))
		//if strings.Contains(resultMessage, "validate") {
		//	c.JSON(http.StatusBadRequest, resultObj{ResultCode:resultCode, ResultMessage:resultMessage})
		//}else{
		//	c.JSON(http.StatusOK, resultObj{ResultCode:resultCode, ResultMessage:resultMessage, Results:resultData})
		//}
		c.JSON(http.StatusOK, resultObj{ResultCode:resultCode, ResultMessage:resultMessage, Results:resultData})
	}()
	data,_ := ioutil.ReadAll(c.Request.Body)
	log.Logger.Debug("Plugin request", log.String("action", action), log.String("agentType", agentType), log.String("param", string(data)))
	var param requestObj
	err := json.Unmarshal(data, &param)
	if err != nil {
		resultCode = "1"
		resultMessage = mid.GetMessageMap(c).RequestJsonUnmarshalError
		return
	}
	if len(param.Inputs) == 0 {
		resultCode = "0"
		resultMessage = fmt.Sprintf(mid.GetMessageMap(c).ParamEmptyError, "inputs")
		return
	}
	for _,v := range param.Inputs {
		tmpAgentType := agentType
		if v.JavaType == "tomcat" || v.Group == "default_tomcat_group" {
			tmpAgentType = "tomcat"
		}
		if v.Password != "" {
			tmpPassword, tmpErr := mid.AesDePassword(v.PasswordGuid, v.PasswordSeed, v.Password)
			if tmpErr == nil {
				v.Password = tmpPassword
			}
		}
		var param m.RegisterParamNew
		var validateMessage,endpointGuid string
		var inputErr error
		tmpStep := 10
		if v.Step != "" {
			tmpStep,_ = strconv.Atoi(v.Step)
			if tmpStep <= 0 {
				tmpStep = 10
			}
		}
		if tmpAgentType == "host" {
			param = m.RegisterParamNew{Type: tmpAgentType, Ip: v.HostIp, Port: "9100", AddDefaultGroup:true, AgentManager:false, FetchMetric:true, DefaultGroupName:v.Group, Step:tmpStep}
		} else {
			param = m.RegisterParamNew{Type: tmpAgentType, Ip: v.InstanceIp, Port: v.Port, Name: v.Instance, User: v.User, Password: v.Password, AgentManager:true, AddDefaultGroup:true, FetchMetric:true, DefaultGroupName:v.Group, Step:tmpStep}
			param.Url = v.Url
			param.Method = v.Method
		}
		if action == "register" {
			validateMessage,endpointGuid,inputErr = AgentRegister(param)
			if validateMessage != "" {
				validateMessage = fmt.Sprintf(mid.GetMessageMap(c).ParamValidateError, validateMessage)
			}
			if validateMessage == "" && inputErr == nil && v.AppLogPaths != "" {
				inputErr = autoAddAppPathConfig(param, v.AppLogPaths)
			}
		} else {
			var endpointObj m.EndpointTable
			if tmpAgentType == "host" {
				endpointObj = m.EndpointTable{Ip: v.HostIp, ExportType: tmpAgentType}
			} else {
				endpointObj = m.EndpointTable{Ip: v.InstanceIp, ExportType: tmpAgentType, Name: v.Instance}
			}
			db.GetEndpoint(&endpointObj)
			if endpointObj.Id > 0 {
				log.Logger.Debug("Export deregister endpoint", log.Int("id", endpointObj.Id), log.String("guid", endpointObj.Guid))
				inputErr = DeregisterJob(endpointObj.Guid)
				endpointGuid = endpointObj.Guid
			}
		}
		if validateMessage != "" || inputErr != nil {
			errorMessage := validateMessage
			if errorMessage == "" {
				errorMessage = fmt.Sprintf(mid.GetMessageMap(c).HandleError, inputErr.Error())
			}
			resultData.Outputs = append(resultData.Outputs, resultOutputObj{CallbackParameter: v.CallbackParameter, ErrorCode: "1", ErrorMessage: errorMessage, Guid: v.Guid, MonitorKey: endpointGuid})
			resultCode = "1"
		}else{
			resultData.Outputs = append(resultData.Outputs, resultOutputObj{CallbackParameter:v.CallbackParameter, ErrorCode:"0", ErrorMessage:"", Guid:v.Guid, MonitorKey:endpointGuid})
		}
	}
}

func AlarmControl(c *gin.Context)  {
	agentType := c.Param("name")
	//if agentType == "java" {
	//	agentType = "tomcat"
	//}
	isStop := false
	action := "start"
	if strings.Contains(c.Request.URL.String(), "stop") {
		isStop = true
		action = "stop"
	}
	var result resultObj
	//var agentPort string
	//for _,v := range m.Config().Agent {
	//	if v.AgentType == agentType {
	//		agentPort = v.Port
	//		break
	//	}
	//}

	data,_ := ioutil.ReadAll(c.Request.Body)
	log.Logger.Info("", log.String("param", string(data)))
	var param requestObj
	err := json.Unmarshal(data, &param)
	if err == nil {
		if len(param.Inputs) == 0 {
			result = resultObj{ResultCode:"0", ResultMessage:fmt.Sprintf(mid.GetMessageMap(c).ParamEmptyError, "inputs")}
			log.Logger.Warn(result.ResultMessage)
			c.JSON(http.StatusOK, result)
			return
		}
		var tmpResult []resultOutputObj
		var resultMessage string
		successFlag := "0"
		for _,v := range param.Inputs {
			//if agentType != "host" && v.Port != "" {
			//	agentPort = v.Port
			//}
			tmpIp := v.HostIp
			if agentType != "host" {
				tmpIp = v.InstanceIp
			}
			err := db.UpdateEndpointAlarmFlag(isStop,agentType,v.Instance,tmpIp,v.Port,v.Pod,v.KubernetesCluster)
			var msg string
			if err != nil {
				msg = fmt.Sprintf("%s %s:%s %s fail,error %v",action, agentType, v.HostIp, v.Instance, err)
				resultMessage = fmt.Sprintf(mid.GetMessageMap(c).HandleError, msg)
				tmpResult = append(tmpResult, resultOutputObj{CallbackParameter:v.CallbackParameter, ErrorCode:"1", ErrorMessage:fmt.Sprintf(mid.GetMessageMap(c).HandleError, msg)})
				successFlag = "1"
			}else{
				msg = fmt.Sprintf("%s %s:%s %s succeed", action, agentType, v.HostIp, v.Instance)
				tmpResult = append(tmpResult, resultOutputObj{CallbackParameter:v.CallbackParameter, ErrorCode:"0", ErrorMessage:""})
			}
			log.Logger.Info(msg)
		}
		result = resultObj{ResultCode:successFlag, ResultMessage:resultMessage, Results:resultOutput{Outputs:tmpResult}}
		log.Logger.Info("result", log.JsonObj("result", result))
		mid.ReturnData(c, result)
	}else{
		result = resultObj{ResultCode:"1", ResultMessage:fmt.Sprintf(mid.GetMessageMap(c).ParamValidateError, err.Error())}
		log.Logger.Error("Param validate fail", log.Error(err))
		c.JSON(http.StatusBadRequest, result)
	}
}

func ExportPingSource(c *gin.Context)  {
	ips := db.GetPingExporterSource()
	mid.ReturnData(c, m.PingExporterSourceDto{Config:ips})
}

func UpdateEndpointTelnet(c *gin.Context)  {
	var param m.UpdateEndpointTelnetParam
	if err := c.ShouldBindJSON(&param); err == nil {
		err = db.UpdateEndpointTelnet(param)
		if err != nil {
			mid.ReturnUpdateTableError(c, "endpoint_telnet", err)
		}else{
			mid.ReturnSuccess(c)
		}
	}else{
		mid.ReturnValidateError(c, err.Error())
	}
}

func GetEndpointTelnet(c *gin.Context)  {
	guid := c.Query("guid")
	if guid == "" {
		mid.ReturnParamEmptyError(c, "guid")
		return
	}
	result,err := db.GetEndpointTelnet(guid)
	if err != nil {
		mid.ReturnQueryTableError(c, "endpoint_telnet", err)
	}else{
		mid.ReturnSuccessData(c, result)
	}
}

func autoAddAppPathConfig(param m.RegisterParamNew, paths string) error {
	tmpPathList := strings.Split(trimListString(paths), ",")
	if len(tmpPathList) == 0 {
		return nil
	}
	hostEndpoint := m.EndpointTable{Ip:param.Ip, ExportType:"host"}
	db.GetEndpoint(&hostEndpoint)
	if hostEndpoint.Id <= 0 {
		return fmt.Errorf("Host endpoint with ip:%s can not find,please register this host first ", param.Ip)
	}
	var businessTables []*m.BusinessUpdatePathObj
	for _,v := range tmpPathList {
		businessTables = append(businessTables, &m.BusinessUpdatePathObj{Path:v, OwnerEndpoint:fmt.Sprintf("%s_%s_%s", param.Name, param.Ip, param.Type)})
	}
	err := db.UpdateAppendBusiness(m.BusinessUpdateDto{EndpointId:hostEndpoint.Id, PathList:businessTables})
	if err != nil {
		log.Logger.Error("Update endpoint business table error", log.Error(err))
		return err
	}
	//err = alarm.UpdateNodeExporterBusinessConfig(hostEndpoint.Id)
	//if err != nil {
	//	log.Logger.Error("Update business config error", log.Error(err))
	//}
	return err
}

type processResult struct {
	ResultCode  string  `json:"resultCode"`
	ResultMessage  string  `json:"resultMessage"`
	Results  processResultOutput  `json:"results"`
}

type processResultOutput struct {
	Outputs  []processResultOutputObj  `json:"outputs"`
}

type processResultOutputObj struct {
	CallbackParameter  string  `json:"callbackParameter"`
	Guid  string  `json:"guid"`
	ErrorCode  string  `json:"errorCode"`
	ErrorMessage  string  `json:"errorMessage"`
	ErrorDetail  string  `json:"errorDetail,omitempty"`
}

type processRequest struct {
	RequestId  string  	`json:"requestId"`
	Inputs  []processRequestObj  `json:"inputs"`
}

type processRequestObj struct {
	Guid  string  `json:"guid"`
	CallbackParameter  string  `json:"callbackParameter"`
	HostIp  string  `json:"host_ip"`
	ProcessName string `json:"process_name"`
	ProcessTag  string `json:"process_tag"`
	DisplayName string `json:"display_name"`
}

func AutoUpdateProcessMonitor(c *gin.Context)  {
	operation := c.Param("operation")
	if operation != "add" && operation != "delete" && operation != "update" {
		mid.ReturnValidateError(c, "Url illegal")
		return
	}
	var param processRequest
	var result processResult
	results := []processResultOutputObj{}
	var err error
	defer func() {
		if err != nil {
			result.ResultCode = "1"
			result.ResultMessage = err.Error()
		}else{
			result.ResultCode = "0"
			result.ResultMessage = "success"
		}
		result.Results= processResultOutput{Outputs:results}
		c.JSON(http.StatusOK, result)
	}()
	if err = c.ShouldBindJSON(&param);err==nil {
		if len(param.Inputs) == 0 {
			return
		}
		for _,input := range param.Inputs {
			subResult,subError := updateProcess(input, operation)
			results = append(results, subResult)
			if subError != nil {
				log.Logger.Error("Handle auto update process fail", log.JsonObj("input", input), log.Error(subError))
				err = subError
			}
		}
	}else{
		return
	}
}

func updateProcess(input processRequestObj,operation string) (result processResultOutputObj,err error) {
	result.Guid = input.Guid
	result.CallbackParameter = input.CallbackParameter
	defer func() {
		if err != nil {
			result.ErrorCode = "1"
			result.ErrorMessage = err.Error()
		}else{
			result.ErrorCode = "0"
			result.ErrorMessage = ""
		}
	}()
	if input.HostIp == "" {
		err = fmt.Errorf("Param host_ip is empty ")
		return result,err
	}
	if input.ProcessName == "" {
		err = fmt.Errorf("Param process_name is empty ")
		return result,err
	}
	if strings.Contains(input.ProcessName, ",") && input.ProcessTag == "" {
		err = fmt.Errorf("Param process_tag cat not empty when process_name is multiple ")
		return result,err
	}
	endpointObj := m.EndpointTable{ExportType:"host", Ip:input.HostIp}
	db.GetEndpoint(&endpointObj)
	if endpointObj.Id <= 0 {
		err = fmt.Errorf("Can not find host endpoint with ip=%s ", input.HostIp)
		return result,err
	}
	var param m.ProcessUpdateDtoNew
	param.EndpointId = endpointObj.Id
	param.ProcessList = append(param.ProcessList, m.ProcessMonitorTable{ProcessName:input.ProcessName, Tags:input.ProcessTag, DisplayName:input.DisplayName})
	err = db.UpdateProcess(param, operation)
	if err != nil {
		err = fmt.Errorf("Update db fail,%s ", err.Error())
		return result,err
	}
	err = db.UpdateNodeExporterProcessConfig(param.EndpointId)
	if err != nil {
		err = fmt.Errorf("Update proxy to remote node_exporter fail,%s ", err.Error())
		return result,err
	}
	return result,err
}

type logMonitorResult struct {
	ResultCode  string  `json:"resultCode"`
	ResultMessage  string  `json:"resultMessage"`
	Results  logMonitorResultOutput  `json:"results"`
}

type logMonitorResultOutput struct {
	Outputs  []logMonitorResultOutputObj  `json:"outputs"`
}

type logMonitorResultOutputObj struct {
	CallbackParameter  string  `json:"callbackParameter"`
	Guid  string  `json:"guid"`
	ErrorCode  string  `json:"errorCode"`
	ErrorMessage  string  `json:"errorMessage"`
	ErrorDetail  string  `json:"errorDetail,omitempty"`
}

type logMonitorRequest struct {
	RequestId  string  	`json:"requestId"`
	Inputs  []logMonitorRequestObj  `json:"inputs"`
}

type logMonitorRequestObj struct {
	Guid  string  `json:"guid"`
	CallbackParameter  string  `json:"callbackParameter"`
	HostIp  string  `json:"host_ip"`
	Path  string `json:"path"`
	Keyword string `json:"keyword"`
	Priority string  `json:"priority"`
}

func AutoUpdateLogMonitor(c *gin.Context)  {
	operation := c.Param("operation")
	if operation != "add" && operation != "delete" && operation != "update" {
		mid.ReturnValidateError(c, "Url illegal")
		return
	}
	var param logMonitorRequest
	var result logMonitorResult
	results := []logMonitorResultOutputObj{}
	var err error
	defer func() {
		if err != nil {
			result.ResultCode = "1"
			result.ResultMessage = err.Error()
		}else{
			result.ResultCode = "0"
			result.ResultMessage = "success"
		}
		result.Results= logMonitorResultOutput{Outputs:results}
		c.JSON(http.StatusOK, result)
	}()
	if err = c.ShouldBindJSON(&param);err==nil {
		if len(param.Inputs) == 0 {
			return
		}
		for _,input := range param.Inputs {
			tmpLogResultObj := logMonitorResultOutputObj{CallbackParameter: input.CallbackParameter,Guid: input.Guid,ErrorCode: "0",ErrorMessage: ""}
			var tmpError error
			for _,v := range strings.Split(input.Path, ",") {
				if v == "" {
					continue
				}
				input.Path = v
				subResult, subError := updateLogMonitor(input, operation)
				if subError != nil {
					tmpLogResultObj = subResult
					tmpError = subError
				}
			}
			results = append(results, tmpLogResultObj)
			if tmpError != nil {
				log.Logger.Error("Handle auto update log monitor fail", log.JsonObj("input", input), log.Error(tmpError))
				err = tmpError
			}
		}
	}else{
		return
	}
}

func updateLogMonitor(input logMonitorRequestObj,operation string) (result logMonitorResultOutputObj,err error) {
	result.Guid = input.Guid
	result.CallbackParameter = input.CallbackParameter
	defer func() {
		if err != nil {
			result.ErrorCode = "1"
			result.ErrorMessage = err.Error()
		}else{
			result.ErrorCode = "0"
			result.ErrorMessage = ""
		}
	}()
	if input.HostIp == "" {
		err = fmt.Errorf("Param host_ip is empty ")
		return result,err
	}
	if input.Path == "" {
		err = fmt.Errorf("Param path is empty ")
		return result,err
	}
	if input.Keyword == "" {
		err = fmt.Errorf("Param keyword is empty ")
		return result,err
	}
	endpointObj := m.EndpointTable{ExportType:"host", Ip:input.HostIp}
	db.GetEndpoint(&endpointObj)
	if endpointObj.Id <= 0 {
		err = fmt.Errorf("Can not find host endpoint with ip=%s ", input.HostIp)
		return result,err
	}
	if input.Priority == "" {
		input.Priority = "low"
	}
	var logMonitorObj m.LogMonitorTable
	logMonitorObj.Path = input.Path
	logMonitorObj.Keyword = input.Keyword
	logMonitorObj.Priority = input.Priority
	logMonitorObj.StrategyId = endpointObj.Id
	err = db.AutoUpdateLogMonitor(&m.UpdateLogMonitor{LogMonitor:[]*m.LogMonitorTable{&logMonitorObj}, Operation:operation})
	if err != nil {
		err = fmt.Errorf("Update log monitor db fail,%s ", err.Error())
		return result,err
	}
	err = db.SendLogConfig(endpointObj.Id, 0, 0)
	if err != nil {
		err = fmt.Errorf("Send log config fail,%s ", err.Error())
		return
	}

	return result,err
}