
import alarmManagement from "@/views/alarm-management";
import alarmHistory from "@/views/alarm-history";
import dashboard from "@/views/dashboard";
import endpointView from "@/views/endpoint-view";
import monitorConfigIndex from "./pages/monitor-config-index";
import endpointManagement from "@/views/monitor-config/endpoint-management";
import groupManagement from "@/views/monitor-config/group-management";
import thresholdManagement from "@/views/monitor-config/threshold-management";
import logManagement from "@/views/monitor-config/log-management";
import resourceLevel from "@/views/monitor-config/resource-level";
import exporter from "@/views/monitor-config/exporter";
import businessMonitor from "@/views/monitor-config/business-monitor";
import metricConfig from "@/views/metric-config";
import viewConfigIndex from "@/views/custom-view/view-config-index";
import viewConfig from "@/views/custom-view/view-config";
import editLineView from "@/views/custom-view/edit-line-view";
import editPieView from "@/views/custom-view/edit-pie-view";
import viewChart from "@/views/custom-view/view-chart";
import portal from "@/views/portal";
import index from "@/views/index";

const router = [
  { path: "/index", name: "index", title: "首页", meta: {}, component: index },
  {
    path: "/alarmManagement",
    name: "alarmManagement",
    title: "告警管理",
    meta: {},
    component: alarmManagement
  },
  {
    path: "/alarmHistory",
    name: "alarmHistory",
    title: "告警历史",
    meta: {},
    component: alarmHistory
  },
  {
    path: "/dashboard",
    name: "dashboard",
    title: "首页",
    meta: {},
    component: dashboard
  },
  {
    path: "/endpointView",
    name: "endpointView",
    title: "对象监控",
    meta: {},
    component: endpointView
  },
  {
    path: "/monitorConfigIndex",
    name: "monitorConfigIndex",
    title: "",
    meta: {},
    component: monitorConfigIndex,
    redirect: "/monitorConfigIndex/endpointManagement",
    children: [
      {
        path: "endpointManagement",
        name: "endpointManagement",
        title: "对象管理",
        meta: {},
        component: endpointManagement
      },
      {
        path: "groupManagement",
        name: "groupManagement",
        title: "组管理",
        meta: {},
        component: groupManagement
      },
      {
        path: "thresholdManagement",
        name: "thresholdManagement",
        title: "阈值配置",
        meta: {},
        component: thresholdManagement
      },
      {
        path: "logManagement",
        name: "logManagement",
        title: "关键字配置",
        meta: {},
        component: logManagement
      },
      {
        path: "resourceLevel",
        name: "resourceLevel",
        title: "资源层级",
        meta: {},
        component: resourceLevel
      },
      {
        path: "exporter",
        name: "exporter",
        title: "exporter",
        meta: {},
        component: exporter
      },
      {
        path: "businessMonitor",
        name: "businessMonitor",
        title: "businessMonitor",
        meta: {},
        component: businessMonitor
      }
    ]
  },
  {
    path: "/metricConfig",
    name: "metricConfig",
    title: "视图配置",
    meta: {},
    component: metricConfig
  },
  {
    path: "/viewConfigIndex",
    name: "viewConfigIndex",
    title: "自定义视图主页",
    meta: {},
    component: viewConfigIndex
  },
  {
    path: "/viewConfig",
    name: "viewConfig",
    title: "自定义视图",
    meta: {},
    component: viewConfig
  },
  {
    path: "/editLineView",
    name: "editLineView",
    title: "自定义视图编辑",
    meta: {},
    component: editLineView
  },
  {
    path: "/editPieView",
    name: "editPieView",
    title: "自定义视图编辑",
    meta: {},
    component: editPieView
  },
  {
    path: "/viewChart",
    name: "viewChart",
    title: "自定义视图放大",
    meta: {},
    component: viewChart
  },
  {
    path: "/portal",
    name: "portal",
    title: "搜索主页",
    meta: {},
    component: portal
  }
];

export default router;
