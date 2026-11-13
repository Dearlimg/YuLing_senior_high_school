import { api, session } from "./api.js";
import { busy, copyText, esc, field, modal, sectionHead, secretDialog, toast } from "./ui.js";
import { navigate, render } from "./app.js";
import { claimState } from "./growth.js";
import { openPoster } from "./poster.js";

const labels={pending:"待核验",active:"已通过",rejected:"未通过",revoked:"已撤销",withdrawn:"已撤回",approved:"已通过，待领取",used:"已领取",expired:"已过期"};

export function classEngagementHTML(hub) {
  return `<section class="class-hub-panel"><div class="section-heading"><div><span class="eyebrow">TOGETHER AGAIN</span><h2>每一次报到，都是一次重逢</h2></div></div><p>${hub.milestone?`我们已达成 <strong>${hub.milestone} 人</strong>班级报到里程碑。`:'从第一声“好久不见”开始，邀请同窗回来看看。'}${hub.next_milestone?` 距离 ${hub.next_milestone} 人里程碑还差 ${hub.next_milestone-hub.count} 位同学。`:""}</p>${hub.next_milestone?`<progress max="${hub.next_milestone}" value="${hub.count}" aria-label="距下一班级报到里程碑的进度"></progress>`:""}<p class="subtle">这是报到里程碑进度，不代表班级覆盖率。</p><div class="growth-actions">${hub.milestone?'<button class="button primary" id="milestone-poster">保存班级纪念海报</button>':""}<button class="button outline" id="apply-organizer">我来做班级召集人</button><a class="text-link" href="/universities?graduation_year=${hub.graduation_year}">我们这一届都去了哪里 →</a></div><h3>班级志愿召集人</h3>${hub.organizers.length?hub.organizers.map(p=>`<article class="organizer-card"><a href="/alumni/${p.alumni_id}"><strong>${esc(p.name)}</strong></a><p>${esc(p.introduction)}</p></article>`).join(""):"<p class=\"subtle\">还没有志愿召集人，欢迎班长、老师和活跃校友一起连接同窗。</p>"}<p class="subtle">召集申请经管理员审核后展示，不代表学校职务或校友身份认证；联系方式继续受保护。</p></section>`;
}

export function bindClassEngagement(root,hub) {
  root.querySelector("#milestone-poster")?.addEventListener("click",()=>openPoster(hub,"class",hub.count,hub.milestone));
  root.querySelector("#apply-organizer")?.addEventListener("click",async()=>{
    if(!session()){navigate(`/login?next=${encodeURIComponent(location.pathname+location.search)}`);return;}
    try {
      const [me,state]=await Promise.all([api("/me"),api("/me/organizer")]);
      if(me.profile.graduation_year!==hub.graduation_year||me.profile.class_name!==hub.class_name){toast("请先在个人中心填写这个班级；召集人申请仅限本人所在班级。");return;}
      const current=state.items[0];
      modal("把同窗重新连接起来",`<p>你将以 ${esc(me.profile.name)} 的身份申请 ${hub.graduation_year} 届 ${esc(hub.class_name)} 志愿召集人。</p>${current?`<p class="subtle">当前申请：${labels[current.status]||esc(current.status)}</p>`:""}<form id="organizer-form"><label class="field">召集介绍<textarea name="introduction" rows="3" maxlength="120" required placeholder="想怎样帮助大家找回同窗？请勿填写手机号和微信。">${esc(current?.introduction||"")}</textarea></label><p class="form-error" role="alert" tabindex="-1" hidden></p><div class="growth-actions"><button class="button primary" type="submit">提交申请，等待审核</button>${current&&["pending","active"].includes(current.status)?'<button class="button outline" type="button" id="withdraw-organizer">撤回我的召集申请</button>':""}</div></form>`);
      const form=document.querySelector("#organizer-form");
      form.onsubmit=e=>{e.preventDefault();busy(form,async()=>{await api("/me/organizer",{method:"POST",body:JSON.stringify({introduction:form.elements.introduction.value})});document.querySelector("#modal").close();toast("申请已提交，通过审核后将在班级页展示。");});};
      document.querySelector("#withdraw-organizer")?.addEventListener("click",()=>busy(form,async()=>{await api(`/me/organizer/${current.id}`,{method:"DELETE"});document.querySelector("#modal").close();await render();toast("已撤回召集申请。");}));
    }catch(error){toast(error.message);}
  });
}

export async function recovery(url) {
  const id=Number(url.searchParams.get("alumni_id"))||"";
  return {
    html:`<div class="wrap page-content recovery-page">${sectionHead("FIND YOUR WAY BACK","找回我的校友卡","未绑定账号的卡片可申请重置编辑码。管理员核验身份后，凭申请凭证领取新码；已绑定账号请登录个人中心。")}
    <div class="recovery-grid"><section class="form-panel"><h2>1. 提交找回申请</h2><form id="request-recovery">${field("校友卡编号","alumni_id",{required:true,type:"number",value:id,placeholder:"公开卡片上的 NO. 编号"})}<label class="field">身份核验说明<textarea name="explanation" required rows="4" maxlength="200" placeholder="说明如何由熟识的班主任、班级召集人或管理员核实本人身份。仅管理员可见，请勿上传证件或填写联系方式。"></textarea></label><p class="subtle">申请凭证仅展示一次，有效期 7 天。请保存申请编号与凭证，并在联系维护者核验时说明编号。提交申请不会自动重置任何人的编辑码。</p><p class="form-error" role="alert" tabindex="-1" hidden></p><button type="submit" class="button primary">提交并保存申请凭证</button></form></section>
    <section class="form-panel"><h2>2. 查询核验结果 / 领取新码</h2><form id="check-recovery">${field("申请编号","request_id",{required:true,type:"number",placeholder:"提交申请后获得的编号"})}${field("申请凭证","receipt",{required:true,type:"password",max:80,placeholder:"你保存的 32 位申请凭证"})}<p class="form-error" role="alert" tabindex="-1" hidden></p><div class="growth-actions"><button type="submit" class="button outline">查询申请状态</button><button type="button" class="button primary" id="complete-recovery" hidden>领取新的编辑码</button></div><p id="recovery-status" role="status"></p></form></section></div></div>`,
    bind(root){
      const request=root.querySelector("#request-recovery"),check=root.querySelector("#check-recovery");
      request.elements.alumni_id.min=1;request.elements.alumni_id.max="999999999999999";
      check.elements.request_id.min=1;check.elements.request_id.max="999999999999999";
      request.onsubmit=e=>{e.preventDefault();busy(request,async()=>{
        const result=await api(`/alumni/${Number(request.elements.alumni_id.value)}/recovery`,{method:"POST",body:JSON.stringify({explanation:request.elements.explanation.value})});
        check.elements.request_id.value=result.id;check.elements.receipt.value=result.receipt;
        secretDialog(`找回申请 #${result.id}`,result.receipt,`请私下保存申请编号 ${result.id} 和此凭证。凭证有效至 ${new Date(result.expires_at).toLocaleDateString("zh-CN")}，请联系网站维护者核验身份。`);
      });};
      check.onsubmit=e=>{e.preventDefault();busy(check,async()=>{
        const result=await api(`/recovery/${Number(check.elements.request_id.value)}/status`,{method:"POST",body:JSON.stringify({receipt:check.elements.receipt.value})});
        root.querySelector("#recovery-status").textContent=`申请状态：${labels[result.status]||result.status}`;
        root.querySelector("#complete-recovery").hidden=result.status!=="approved";
      });};
      root.querySelector("#complete-recovery").onclick=()=>busy(check,async()=>{
        const result=await api(`/recovery/${Number(check.elements.request_id.value)}/complete`,{method:"POST",body:JSON.stringify({receipt:check.elements.receipt.value})});
        claimState(result.id,{editCode:result.edit_code,recovered:true});check.reset();navigate(`/alumni/${result.id}`);toast("编辑码已重置，请私下保存新码。");
      });
      check.addEventListener("input",()=>{root.querySelector("#complete-recovery").hidden=true;});
    },
  };
}

export function engagementAdminHTML(view,data) {
  const organizer=view==="organizers";
  return `<p class="subtle">${organizer?"审核志愿召集人介绍与本人班级；资料换班后原班级页将不再展示。":"请通过已知校友关系核实本人身份，并确认申请编号属于该本人，再批准找回。申请说明本身不能证明身份；审核记录只对管理员可见。"}</p><div class="admin-rows">${data.items.map(p=>`<article class="admin-row"><div><a href="/alumni/${p.alumni_id}"><strong>${esc(p.name)} · 校友卡 #${p.alumni_id}</strong></a><span class="admin-status">${labels[p.status]||esc(p.status)}</span><p>${organizer?`${p.graduation_year} 届 ${esc(p.class_name)} · ${esc(p.introduction)}`:`申请 #${p.id} · ${esc(p.explanation)}`}</p></div><div class="growth-actions">${p.status==="pending"?`<button class="button primary" data-review="${p.id}" data-status="${organizer?"active":"approved"}">${organizer?"通过召集申请":"已核实本人，批准找回"}</button><button class="button outline" data-review="${p.id}" data-status="rejected">不通过</button>`:organizer&&p.status==="active"?`<button class="button outline" data-review="${p.id}" data-status="revoked">撤销召集人</button>`:""}</div></article>`).join("")||'<p class="subtle">暂无申请。</p>'}</div>`;
}

export function bindEngagementAdmin(root,view,adminAPI){
  root.querySelectorAll("[data-review]").forEach(button=>button.onclick=()=>{
    modal("记录审核依据",`<form id="engagement-review"><label class="field">核验记录<textarea name="reason" rows="3" maxlength="200" required placeholder="说明已完成的本人身份核验或审核依据"></textarea></label><p class="form-error" role="alert" tabindex="-1" hidden></p><button class="button primary" type="submit">确认审核结果</button></form>`);
    const form=document.querySelector("#engagement-review");form.onsubmit=e=>{e.preventDefault();busy(form,async()=>{
      await adminAPI(`/admin/${view}/${button.dataset.review}`,{method:"PUT",body:JSON.stringify({status:button.dataset.status,reason:form.elements.reason.value})});
      document.querySelector("#modal").close();await render();toast("审核结果已保存。");
    });};
  });
}
