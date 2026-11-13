import { growthDashboard } from "./growth-admin.js";
import { api, session } from "./api.js";
import { icon } from "./icons.js";
import { busy, closeModal, dateLabel, empty, esc, modal, pagination, sectionHead, toast } from "./ui.js";
import { navigate, render } from "./app.js";

// Bootstrap credentials only live in this module's memory, never browser storage.
let bootstrapToken = "";
const adminAPI = (path, options = {}) => api(path, {...options, headers: bootstrapToken ? {...options.headers, Authorization:`Bearer ${bootstrapToken}`} : options.headers});
const statuses = {active:"公开",hidden:"已下架",deleted:"已软删除"};

function adminGate(message = "") {
  return {
    html:`<div class="wrap page-content admin-gate">${sectionHead("COMMUNITY MANAGEMENT","社区管理","使用已授权的管理员账号登录。首次授权可使用部署时配置的管理 Token。")}<div class="composer-card"><p>${esc(message || "管理员可以管理用户权限与公开内容。")}</p><a href="/login?next=/admin" class="button primary">登录管理员账号</a><details class="bootstrap-gate"><summary>使用管理 Token 首次授权</summary><form id="bootstrap-form"><label class="field">管理 Token<input type="password" name="token" required minlength="32" autocomplete="off" placeholder="输入 ADMIN_TOKEN"/></label><p class="subtle">仅在本次页面会话中使用，刷新后需重新输入。</p><p class="form-error" role="alert" tabindex="-1" hidden></p><button class="button outline" type="submit">进入管理页面</button></form></details></div></div>`,
    bind(root) {
      root.querySelector("#bootstrap-form").onsubmit = event => {
        event.preventDefault(); const form = event.currentTarget;
        busy(form,async()=>{
          const token = form.elements.token.value.trim();
          await api("/admin/users?page=1&page_size=1",{headers:{Authorization:`Bearer ${token}`}});
          bootstrapToken = token; form.reset(); navigate("/admin");
        });
      };
    },
  };
}

export async function admin(url) {
  let me = null;
  if (!bootstrapToken) {
    if (!session()) return adminGate();
    me = await api("/me");
    if (!me.is_admin) return adminGate("当前账号没有管理员权限，请由现有管理员授权。");
  }
  const view = ["users","content","audit","growth"].includes(url.searchParams.get("view")) ? url.searchParams.get("view") : "users";
  const kind = url.searchParams.get("kind") === "photo" ? "photo" : "post";
  const status = statuses[url.searchParams.get("status")] ? url.searchParams.get("status") : "";
  const page = Math.max(1,Number(url.searchParams.get("page")) || 1);
  const keyword = url.searchParams.get("keyword") || "";
  const target = Number(url.searchParams.get("target"));
  const path = changes => `/admin?${new URLSearchParams({view,kind,status,keyword,...changes})}`;
  let data;
  try {
    if (view === "growth") data = await adminAPI(`/admin/growth?${new URLSearchParams(Object.fromEntries(["days","source","campaign","graduation_year","class_name"].map(key=>[key,url.searchParams.get(key)||""]).filter(([,value])=>value)))}`);
    else if (view === "users") data = await adminAPI(`/admin/users?${new URLSearchParams({keyword,page,page_size:12})}`);
    else if (view === "audit") data = await adminAPI(`/admin/audit?page=${page}&page_size=20`);
    else if (Number.isSafeInteger(target) && target > 0) {
      const row = await adminAPI(`/admin/${kind}/${target}`); data = {items:[row],total:1};
    } else data = await adminAPI(`/admin/content/${kind}?${new URLSearchParams({keyword,status,page,page_size:12})}`);
  } catch(error) {
    if ([401,403].includes(error.status)) { bootstrapToken=""; return adminGate(error.message); }
    throw error;
  }
  let content;
  if (view === "growth") content = growthDashboard(data,url);
  else if (view === "users") content = `<form class="post-search" id="admin-search"><label class="sr-only" for="admin-keyword">查找账号或姓名</label><input id="admin-keyword" name="keyword" maxlength="100" placeholder="搜索账号或校友姓名…" value="${esc(keyword)}"/><button class="button primary" type="submit">查找用户</button></form><p class="subtle">共 ${data.total} 个用户。授权后可管理用户和社区内容，撤权会在下一次请求时生效。</p><div class="admin-rows">${data.items.map(u=>`<article class="admin-row"><div><strong>${esc(u.name || u.account)}</strong><span class="admin-status">${u.is_admin ? "管理员" : "普通用户"}</span><p>账号：${esc(u.account)} · User ID：${u.id} · 校友卡：${u.alumni_id}</p></div><button class="button outline" data-role-user="${u.id}" ${u.id === me?.user_id ? "disabled" : ""}>${u.is_admin ? "撤销管理员" : "设为管理员"}</button></article>`).join("") || empty("未找到用户","换一个账号或姓名试试。")}</div>${pagination(page,data.total,12)}`;
  else if (view === "content") content = `<div class="admin-filters"><div class="category-filters"><button class="chip ${kind==="post"?"active":""}" data-kind="post">帖子</button><button class="chip ${kind==="photo"?"active":""}" data-kind="photo">照片</button></div><label>状态 <select id="admin-status"><option value="">全部</option>${Object.entries(statuses).map(([key,label])=>`<option value="${key}" ${status===key?"selected":""}>${label}</option>`).join("")}</select></label></div><form class="post-search" id="admin-search"><label class="sr-only" for="admin-keyword">搜索内容</label><input id="admin-keyword" name="keyword" maxlength="100" placeholder="搜索帖子正文或照片说明…" value="${esc(keyword)}"/><button class="button primary" type="submit">查找内容</button></form>${target > 0 ? `<a class="text-link" href="${path({view:"content"})}">返回内容列表</a>` : ""}<p class="subtle">共 ${data.total} 条内容。软删除保留原始记录与审计，公开入口将不可访问。</p><div class="admin-rows">${data.items.map(item=>`<article class="admin-row"><div><strong>${kind==="photo"?"照片":"帖子"} #${item.id}</strong><span class="admin-status ${item.status}">${statuses[item.status] || esc(item.status)}</span><p class="admin-summary">${esc(item.summary)}</p><small>${dateLabel(item.created_at)}</small></div><div class="admin-row-actions">${item.status!=="deleted" ? `${item.status==="active"?`<a href="/${kind==="photo"?"moments?photo":"forum?post"}=${item.id}" class="text-link">查看</a>`:""}<button class="button outline" data-moderate="${item.id}" data-action-value="${item.status==="active"?"hidden":"active"}">${item.status==="active"?"下架":"恢复公开"}</button><button class="button danger" data-delete-content="${item.id}">软删除</button>`:'<span class="subtle">原始记录已保留</span>'}</div></article>`).join("") || empty("没有符合条件的内容","试试其他关键词或状态。")}</div>${pagination(page,data.total,12)}`;
  else content = `<p class="subtle">按时间倒序记录权限变更与内容管理操作。操作人 0 表示管理 Token 或历史操作。</p><div class="admin-rows">${data.items.map(log=>`<article class="admin-row"><div><strong>${esc({grant_admin:"授予管理员",revoke_admin:"撤销管理员",active:"恢复公开",hidden:"下架",deleted:"软删除"}[log.Action] || log.Action)}</strong><span class="admin-status">${esc(log.Kind)} #${log.TargetID}</span><p>${esc(log.Reason)}</p><small>操作人 User ID：${log.ActorUserID || 0} · ${dateLabel(log.CreatedAt)}</small></div></article>`).join("") || empty("暂无操作记录","管理操作会在这里留下记录。")}</div><div class="pagination">${page>1?`<button class="button outline" data-page="${page-1}">上一页</button>`:""}<span>第 ${page} 页</span>${data.items.length===20?`<button class="button outline" data-page="${page+1}">下一页</button>`:""}</div>`;
  return {
    html:`<div class="wrap page-content admin-page">${sectionHead("COMMUNITY MANAGEMENT","让社区保持温暖",bootstrapToken?"管理 Token 会话":"当前管理员："+esc(me.account),'<button class="button outline" id="leave-admin">退出管理</button>')}<nav class="admin-tabs" aria-label="管理分类">${[["users","用户权限"],["content","内容管理"],["audit","操作审计"],["growth","增长数据"]].map(([key,label])=>`<a class="chip ${view===key?"active":""}" href="${path({view:key,page:1})}">${label}</a>`).join("")}</nav>${content}</div>`,
    bind(root) {
      root.querySelector("#growth-filters")?.addEventListener("submit",event=>{event.preventDefault();const params=new URLSearchParams(new FormData(event.currentTarget));for(const [key,value] of [...params])if(!value)params.delete(key);params.set("view","growth");navigate(`/admin?${params}`);});
      root.querySelector("#leave-admin").onclick=()=>{bootstrapToken="";navigate("/me");};
      root.querySelectorAll("[data-page]").forEach(b=>b.onclick=()=>navigate(path({page:b.dataset.page})));
      root.querySelectorAll("[data-kind]").forEach(b=>b.onclick=()=>navigate(path({kind:b.dataset.kind,page:1})));
      root.querySelector("#admin-status")?.addEventListener("change",event=>navigate(path({status:event.target.value,page:1})));
      root.querySelector("#admin-search")?.addEventListener("submit",event=>{event.preventDefault();navigate(path({keyword:event.currentTarget.elements.keyword.value.trim(),page:1}));});
      root.querySelectorAll("[data-role-user]").forEach(button=>button.onclick=()=>{
        const user = data.items.find(u=>u.id===Number(button.dataset.roleUser));
        actionDialog(user.is_admin?"撤销管理员":"设为管理员",`用户：${user.name || user.account}（${user.account} / #${user.id}）`,async reason=>{
          await adminAPI(`/admin/users/${user.id}/role`,{method:"PUT",body:JSON.stringify({is_admin:!user.is_admin,reason})});
        });
      });
      root.querySelectorAll("[data-moderate], [data-delete-content]").forEach(button=>button.onclick=()=>{
        const deleting = !!button.dataset.deleteContent;
        const id = Number(button.dataset.deleteContent || button.dataset.moderate);
        const item = data.items.find(i=>i.id===id);
        const action = deleting?"deleted":button.dataset.actionValue;
        actionDialog(deleting?"软删除内容":action==="active"?"恢复公开":"下架内容",`${kind==="photo"?"照片":"帖子"} #${id}：${item.summary}`,async reason=>{
          await adminAPI(`/admin/${kind}/${id}`,{method:deleting?"DELETE":"PUT",body:JSON.stringify({action,reason})});
        },deleting);
      });
    },
  };
}

function actionDialog(title,description,save,deleting=false) {
  modal(title,`<p class="modal-description admin-summary">${esc(description)}</p>${deleting?'<p class="subtle">内容将从所有公开入口撤下，数据库原始记录及互动数据保留。</p>':""}<form id="admin-action-form"><label class="field">操作原因<textarea name="reason" rows="3" maxlength="200" required placeholder="填写原因，方便审计和后续核对"></textarea></label><p class="form-error" role="alert" tabindex="-1" hidden></p><button class="button ${deleting?"danger":"primary"} full-width" type="submit">确认${title}</button></form>`);
  const form = document.querySelector("#admin-action-form");
  form.onsubmit=event=>{event.preventDefault();busy(form,async()=>{await save(form.elements.reason.value.trim());closeModal();await render();toast("管理操作已完成");});};
}
