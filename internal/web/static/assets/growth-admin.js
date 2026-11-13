import { esc, field } from "./ui.js";

export function growthDashboard(data,url) {
  const c=data.counts;
  const percent=n=>n===null?"—":`${(n*100).toFixed(1)}%`;
  const metrics=[
    ["成功领卡 UV",c.registration_success||0,"由领卡服务端记录，与档案同一事务提交。"],
    ["登记转化率",percent(data.registration_rate),`${c.registration_success||0} 领卡 UV / ${c.registration_view||0} 登记页 UV`],
    ["表单完成率",percent(data.completion_rate),`${c.registration_success||0} 领卡 UV / ${c.registration_start||0} 开始填写 UV`],
    ["分享率",percent(data.share_rate),`${c.share||0} 分享操作 UV / ${c.registration_success||0} 领卡 UV`],
    ["分享回流率",percent(data.return_rate),`${c.share_visit||0} 回访 UV / ${c.share||0} 分享操作 UV`],
    ["分享登记率",percent(data.share_registration_rate),`${c.share_registration||0} 分享来源领卡 UV / ${c.share_visit||0} 回访 UV`],
  ];
  return `<form id="growth-filters" class="growth-filters"><label class="field">统计范围<select name="days">${[1,7,30,90].map(n=>`<option value="${n}" ${n===data.days?"selected":""}>最近 ${n} 天</option>`).join("")}</select></label>${field("来源","source",{max:48,value:url.searchParams.get("source"),placeholder:"如 class_invite"})}${field("活动","campaign",{max:48,value:url.searchParams.get("campaign"),placeholder:"如 reunion"})}${field("届别","graduation_year",{type:"number",value:url.searchParams.get("graduation_year")})}${field("班级","class_name",{max:30,value:url.searchParams.get("class_name"),placeholder:"如 高三1班"})}<button class="button primary" type="submit">查看增长数据</button></form><div class="growth-metrics">${metrics.map(([title,value,note])=>`<article class="growth-metric"><span>${esc(title)}</span><strong>${value}</strong><p>${note}</p></article>`).join("")}</div><div class="inline-notice">统计覆盖本次上线后的免账号领卡链路。UV 按匿名浏览器去重，跨设备会重复、清理存储会重置。分享率合并复制链接、唤起系统分享和点击保存海报，同一浏览器只计一次；不表示实际发送或保存成功。长按保存、未执行脚本及埋点被拦截的访问无法完整统计。</div><p class="subtle">来源、活动、届别和班级均为精确筛选。比率使用所选时间段内各事件 UV，不是同一批用户的队列转化，可能超过 100%；无分母显示“—”。班级页展示有效档案人数；尚无可触达总人数，不估算班级覆盖率。</p>`;
}
