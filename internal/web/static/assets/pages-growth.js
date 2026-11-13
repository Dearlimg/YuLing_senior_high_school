import { api, requestKey } from "./api.js";
import { alumniCard as listCard, busy, copyText, empty, esc, field, modal, pagination, sectionHead, toast } from "./ui.js";
import { navigate } from "./app.js";
import { claimState, contextFor, shareURL, shareContext, track } from "./growth.js";
import { openPoster } from "./poster.js";

export async function quickClaim(url, card) {
  const year = Number(url.searchParams.get("graduation_year")) || "";
  const person = {
    identity: url.searchParams.get("identity") === "student" ? "student" : "alumni",
    graduation_year: year, class_name: (url.searchParams.get("class_name") || "").slice(0,30), avatar_color:"red",
  };
  const meta = await api("/directory-meta").catch(() => null);
  return {
    html:`<div class="wrap page-content registration-page quick-claim">${sectionHead("YOUR PLACE IN OUR STORY", "领取我的校友卡，找回同班同学", "看看榆中人都去了哪里，也让老同学找到你。")}
      <div class="claim-steps"><span class="active">01 领取校友卡</span><span>02 邀请同班同学</span><span>03 补充近况 · 可稍后</span></div>
      <div class="registration-layout"><section class="form-panel"><span class="eyebrow">从一个名字开始重逢</span><h2>你好，榆中人。</h2><p class="subtle">只需填写基本信息，无需设置账号密码。</p>
      ${meta?.total ? `<p class="claim-proof">已有 <strong>${meta.total}</strong> 位校友与同学报到，覆盖 <strong>${meta.years.length}</strong> 个届别。</p>` : ""}
      ${person.class_name && year ? `<div class="inline-notice">你正向 ${year} 届 ${esc(person.class_name)} 报到，届别与班级均可修改。</div>` : ""}
      <form id="quick-claim-form"><div class="identity-tabs"><button type="button" data-identity="alumni">我是校友</button><button type="button" data-identity="student">我还在读</button></div>
      <input name="identity" type="hidden" value="${person.identity}"/><div class="form-grid">${field("真实姓名","name",{required:true,max:30,placeholder:"让同学认出你"})}${field("毕业年份","graduation_year",{required:true,type:"number",value:year,placeholder:"如 2018"})}<div class="full-width">${field("班级","class_name",{max:30,value:person.class_name,placeholder:"如 高三1班",help:"填写班级，更容易被同班同学找到；在读同学必填。"})}</div></div>
      <div class="inline-notice">姓名、届别和班级将公开；手机号、微信默认不公开，领卡后可逐项选择。</div>
      <label class="checkbox-field consent"><input name="consent" type="checkbox" required/><span>我已阅读<a href="/privacy" target="_blank">隐私与公开说明</a>，同意公开所填基本资料。</span></label>
      <p class="form-error" role="alert" tabindex="-1" hidden></p><button class="button primary full-width large" type="submit">领取我的校友卡 →</button><p class="form-submit-note">大学、城市和寄语等近况，领卡后随时补充。</p></form>
      <p class="subtle">已有校友卡？<a href="/login">登录管理</a>，或前往自己的卡片凭编辑码修改。</p></section>
      <aside class="registration-aside"><div class="sticky-aside"><div class="preview-heading"><span class="eyebrow">YOUR ALUMNI ID</span><span>实时预览</span></div><div id="card-preview">${card(person,true)}</div><div class="aside-note"><h3>从此山海，皆有同窗。</h3><p>领一张属于自己的校友卡，<br>再把这份重逢，带给同班同学。</p></div></div></aside></div></div>`,
    bind(root) {
      const form = root.querySelector("#quick-claim-form");
      const key = requestKey();
      let started = false;
      const start = () => { if (!started) {started=true;track("registration_start",person);} };
      const snapshot = () => ({...person,...Object.fromEntries(new FormData(form)),graduation_year:Number(form.elements.graduation_year.value)||0});
      const update = () => {
        const p = snapshot(), student = p.identity === "student";
        form.elements.graduation_year.min = student ? new Date().getFullYear() : 1903;
        form.elements.graduation_year.max = new Date().getFullYear() + (student ? 6 : 0);
        form.elements.graduation_year.closest("label").firstChild.textContent = student ? "预计毕业年份" : "毕业年份";
        form.elements.class_name.required = student;
        root.querySelectorAll("[data-identity]").forEach(b => {b.classList.toggle("selected",b.dataset.identity===p.identity);b.setAttribute("aria-pressed",String(b.dataset.identity===p.identity));});
        root.querySelector("#card-preview").innerHTML = card(p,true);
      };
      track("registration_view",person);
      form.addEventListener("input",()=>{start();update();});
      root.querySelectorAll("[data-identity]").forEach(b=>b.onclick=()=>{form.elements.identity.value=b.dataset.identity;start();update();});
      update();
      form.onsubmit = event => {
        event.preventDefault();
        if (!form.reportValidity()) return;
        busy(form,async()=>{
          const data = snapshot(); delete data.consent;
          start();
          let result;
          try { result = await api("/alumni/claim",{method:"POST",headers:{"Idempotency-Key":key},body:JSON.stringify({...data,attribution:contextFor(data)})}); }
          catch(error) {track("registration_failure",data);throw error;}
          claimState(result.id,{editCode:result.edit_code});
          navigate(`/alumni/${result.id}?welcome=1`);
        });
      };
    },
  };
}

export function successHTML(person) {
  const state = claimState(person.id);
  if (!state) return "";
  return `<section class="claim-success"><div><span class="eyebrow">WELCOME HOME</span><h2>校友卡已领取，欢迎回家。</h2><p>先保存编辑码，再邀请同班同学一起来报到。</p></div><div class="edit-code-box"><strong>你的编辑码 · 请私下保存</strong><code>${esc(state.editCode)}</code><button class="button outline" id="copy-edit-code">复制编辑码</button><small>仅本次页面会话可见，刷新后不再展示。请勿放进海报或发给别人。</small></div><div class="growth-actions">${person.class_name ? '<button class="button primary" id="welcome-invite">邀请同班同学</button>' : ""}<button class="button outline" id="welcome-save-card">保存校友卡海报</button><button class="button outline" id="complete-profile">让同学更容易找到我 · 补充近况</button><a class="button outline" href="/login?claim=1&alumni_id=${person.id}">可选：绑定账号，方便下次登录</a></div></section>`;
}

export function shareActionsHTML(person) {
  return `<div class="growth-share"><div class="growth-actions">${person.class_name ? '<button class="button primary" id="invite-class">邀请同班同学</button><button class="button outline" id="class-poster">生成班级召集海报</button>' : '<p class="subtle">补充班级后，可以邀请同班同学一起报到。</p>'}<button class="button outline" id="save-card">保存校友卡海报</button>${person.class_name ? `<a class="text-link" href="/class?${new URLSearchParams({graduation_year:person.graduation_year,class_name:person.class_name})}">看看同班都有谁 →</a>` : '<a class="text-link" href="/register">我也要领取校友卡 →</a>'}</div><p class="subtle">海报仅含公开资料；分享前可预览。</p></div>`;
}

export async function editWithCode(person, root) {
  const open = async code => {
    const owner = await api(`/alumni/${person.id}/unlock`,{method:"POST",body:JSON.stringify({edit_code:code})});
    const {register} = await import("./pages-profile.js");
    const page = await register(new URL(location.href),owner,code);
    document.querySelector("#modal").close();
    root.innerHTML = page.html; page.bind(root); window.scrollTo(0,0);
  };
  const code = claimState(person.id)?.editCode;
  if (code) { try {await open(code);} catch(error) {toast(error.message);} return; }
  modal("管理我的校友卡",`<p class="subtle">已绑定账号的校友卡，请<a href="/login?next=/me">登录个人中心</a>。未绑定的卡片可凭编辑码修改。</p><form id="unlock-card-form">${field("编辑码","code",{required:true,type:"password",max:80})}<p class="form-error" role="alert" tabindex="-1" hidden></p><button class="button primary" type="submit">验证并修改</button></form>`);
  const form = document.querySelector("#unlock-card-form");
  form.onsubmit=event=>{event.preventDefault();busy(form,()=>open(form.elements.code.value.trim()));};
}

export function bindGrowthProfile(root,person) {
  root.querySelector("#welcome-invite")?.addEventListener("click",()=>inviteClass(person));
  root.querySelector("#welcome-save-card")?.addEventListener("click",()=>openPoster(person,"card"));
  root.querySelector("#copy-edit-code")?.addEventListener("click",()=>{copyText(claimState(person.id).editCode);track("edit_code_copy",person);});
  root.querySelector("#complete-profile")?.addEventListener("click",()=>editWithCode(person,root));
  root.querySelector("#invite-class")?.addEventListener("click",()=>inviteClass(person));
  root.querySelector("#class-poster")?.addEventListener("click",()=>classPoster(person));
  root.querySelector("#save-card")?.addEventListener("click",()=>openPoster(person,"card"));
}

async function classCount(person) {
  return api(`/alumni?${new URLSearchParams({graduation_year:person.graduation_year,class_name:person.class_name,page_size:1})}`);
}

async function classPoster(person) {
  try { const result = await classCount(person); await openPoster(person,"class",result.total); }
  catch(error) {toast(error.message);}
}

async function inviteClass(person) {
  try {
    const result = await classCount(person), url = shareURL(person);
    const text = `我在榆中校友之家领到了校友卡。${person.graduation_year}届${person.class_name}已有${result.total}位同学报到，来看看都有谁。`;
    modal("把这份重逢，带给同班同学",`<p class="invite-copy">${esc(text)}</p><label class="field">班级邀请链接<input readonly id="class-share-link" value="${esc(url.href)}"/></label><div class="growth-actions"><button class="button primary" id="copy-class-link">复制邀请文案和链接</button><button class="button outline" id="native-share">使用系统分享</button></div><p class="subtle">微信内可复制文案发给同学，或保存班级召集海报后转发。复制与唤起分享不代表发送成功。</p>`);
    document.querySelector("#copy-class-link").onclick=()=>{copyText(`${text}\n${url.href}`);track("share_click",person,shareContext(url));};
    document.querySelector("#native-share").onclick=async()=>{
      if (!navigator.share) {toast("当前浏览器暂不支持系统分享，请复制邀请链接。");return;}
      track("share_click",person,shareContext(url));
      try {await navigator.share({title:"同班同学，等你报到",text,url:url.href});} catch(error) {if(error.name!=="AbortError")toast("分享未完成，可以复制链接后发送。");}
    };
  } catch(error) {toast(error.message);}
}

export async function classPage(url) {
  const year = Number(url.searchParams.get("graduation_year"));
  const name = (url.searchParams.get("class_name") || "").trim();
  if (!Number.isInteger(year) || year<1903 || year>new Date().getFullYear()+6 || !name || Array.from(name).length>30 || name==="__unassigned__") {
    return {html:`<div class="wrap page-content">${empty("这份班级邀请不完整","请从校友卡重新生成邀请链接，或去通讯录找找同学。",'<a class="button primary" href="/directory">查看通讯录</a>')}</div>`};
  }
  const page = Math.max(1,Number(url.searchParams.get("page")) || 1);
  const person = {graduation_year:year,class_name:name};
  const result = await api(`/alumni?${new URLSearchParams({...person,page,page_size:12})}`);
  const params = new URLSearchParams(url.search);params.delete("page");params.set("graduation_year",year);params.set("class_name",name);
  if (year>new Date().getFullYear()) params.set("identity","student");
  const register = `/register?${params}`;
  return {
    html:`<div class="wrap page-content class-page"><section class="class-reunion"><span class="eyebrow">OUR CLASS · OUR SHARED STORY</span><p class="class-year">${year} 届 · ${esc(name)}</p><h1>好久不见，<br>这一次，等你报到。</h1><p>已有 <strong>${result.total}</strong> 位同学在这里相聚。<br>来看看都有谁，也让同学找到你。</p><div class="growth-actions"><a class="button primary large" href="${esc(register)}">我也是这个班的，立即报到 →</a><button class="button outline" id="class-poster">生成班级召集海报</button></div><small>自由浏览，无需登录；报到时可修改届别与班级。</small></section><div class="section-heading"><div><span class="eyebrow">NICE TO SEE YOU AGAIN</span><h2>同班都有谁 · ${result.total} 人</h2></div><a href="/directory" class="text-link">看看其他届别 →</a></div><div class="recent-alumni class-members">${result.items.map(listCard).join("") || empty("第一位报到的同学，会是谁？","写下自己的名字，邀请老同学一起重逢。")}</div>${pagination(page,result.total,12)}</div>`,
    bind(root) {
      root.querySelector("#class-poster").onclick=()=>openPoster(person,"class",result.total);
      root.querySelectorAll("[data-page]").forEach(b=>b.onclick=()=>{const next=new URL(url);next.searchParams.set("page",b.dataset.page);navigate(next.pathname+next.search);});
    },
  };
}
