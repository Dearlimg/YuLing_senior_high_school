import { api, requestKey, saveSession, session } from "./api.js";
import { icon } from "./icons.js";
import {
  avatar,
  busy,
  colors,
  esc,
  field,
  modal,
  sectionHead,
  secretDialog,
  toast,
} from "./ui.js";
import { navigate } from "./app.js";

function alumniCard(person, preview = false) {
  const student = person.identity === "student";
  return `<div class="identity-card ${preview ? "preview-card" : ""}"><div class="identity-card-top"><img src="/assets/mark.svg" width="40" height="40" alt=""/><span>陕西省榆林中学<small>YULIN HIGH SCHOOL</small></span><span class="identity-card-type">${student ? "在读同学卡" : "电子校友卡"}</span></div><div class="identity-person">${avatar(person, "big")}<div><h2>${esc(person.name || "你的姓名")}</h2><p>${person.graduation_year || "20XX"} 届 ${esc(person.class_name || "")} ${student ? "<span>· 在读</span>" : ""}</p></div></div><div class="identity-school"><small>${student ? "理想大学 · MY DREAM" : "大学 / 就读院校"}</small><strong>${esc((student ? person.dream_university : person.university) || "人生下一程，期待你的故事")}</strong>${person.major ? `<p>${esc(person.major)}</p>` : ""}</div><div class="identity-card-bottom"><span>怀天下 · 求真知 · 报国家</span><span>${preview ? "SINCE 1903" : `NO. ${String(person.id).padStart(6, "0")}`}</span></div><span class="identity-watermark" aria-hidden="true">1903</span></div>`;
}

export async function register(
  url,
  initial = null,
  editCode = "",
  draft = null,
) {
  if (!initial && session()) return personal(url);
  const person = draft ||
    initial || {
      identity:
        url.searchParams.get("identity") === "student" ? "student" : "alumni",
      avatar_color: "red",
      contact_public: true,
    };
  const student = person.identity === "student";
  const editing = Boolean(initial);
  return {
    html: `<div class="wrap page-content registration-page">${sectionHead("ALUMNI REGISTRATION", editing ? "把近况，写进新的篇章" : "登记一份近况，珍藏一张校友卡", "每个人的故事，都是榆中故事的一部分。很高兴，在这里再次认识你。")}
    <div class="registration-layout"><section class="form-panel"><form id="registration-form"><input type="hidden" name="identity" value="${person.identity}"/><input type="hidden" name="avatar_color" value="${person.avatar_color}"/>
    ${!editing ? `<div class="form-section-heading"><span>00</span><h2>创建登录账号</h2><a href="/login">已有账号？登录</a></div><div class="form-grid">${field("账号", "account", {required:true, value:person.account, max:32, placeholder:"4～32 位字母、数字或下划线"})}${field("密码", "password", {required:true, type:"password", max:72, placeholder:"至少 8 字节"})}</div><p class="subtle">账号用于登录和管理校友卡，请妥善保存。</p>` : ""}<div class="identity-tabs"><button type="button" data-identity="alumni" class="${!student ? "selected" : ""}" aria-pressed="${!student}">${icon("card")}我是校友</button><button type="button" data-identity="student" class="${student ? "selected" : ""}" aria-pressed="${student}">${icon("school")}我还在读</button></div>
    <div class="form-section-heading"><span>01</span><h2>让我们重新认识你</h2><small>基础信息</small></div>
    <div class="form-grid">${field("真实姓名", "name", { required: true, value: person.name, placeholder: "你的真实姓名", max: 30 })}${field(student ? "预计毕业年份" : "毕业年份", "graduation_year", { required: true, value: person.graduation_year, placeholder: student ? "如 2027" : "如 2018", type: "number" })}${field(student ? "当前班级" : "毕业班级", "class_name", { required: student, value: person.class_name, placeholder: "如 高三1班", max: 30 })}${field("现在所在城市", "city", { value: person.city, placeholder: "如 西安 / 北京" })}${field(student ? "理想大学" : "大学 / 就读院校", student ? "dream_university" : "university", { value: student ? person.dream_university : person.university, placeholder: "如 西安交通大学", max: 100 })}${field("专业", "major", { value: person.major, placeholder: "如 电气工程" })}<label class="field">学历<span class="optional">选填</span><select name="education">${["未填写", "高中", "大专", "本科", "硕士", "博士", "其他"].map((x) => `<option value="${x === "未填写" ? "" : x}" ${person.education === x ? "selected" : ""}>${x}</option>`).join("")}</select></label>${field("职位", "position", { value: person.position, placeholder: "如 工程师 / 学生" })}<div class="full-width">${field("工作单位", "company", { value: person.company, placeholder: "公司、学校或单位名称", max: 100 })}</div></div>
    <div class="form-section-heading"><span>02</span><h2>留一扇联系的窗</h2><small>联系方式</small></div><div class="inline-notice">${icon("lock")}基本资料将公开展示。联系方式仅用于档案维护，当前不向普通访问者展示。</div><div class="form-grid">${field("手机号", "phone", { value: person.phone, placeholder: "中国大陆手机号或 + 国际号码", type: "tel", max: 20 })}${field("微信号", "wechat", { value: person.wechat, placeholder: "选填" })}<div class="full-width">${field("邮箱", "email", { value: person.email, placeholder: "用于资料联系，默认不公开", type: "email", max: 100 })}</div></div><label class="checkbox-field"><input type="checkbox" name="contact_public" ${person.contact_public ? "checked" : ""}/><span>未来开放校友认证后，允许已验证校友查看我的手机号和微信<small>可随时关闭。登记不等于身份认证，邮箱始终不公开。</small></span></label>
    <div class="form-section-heading"><span>03</span><h2>为你的校友卡添一笔</h2><small>个性设置</small></div><div class="field"><span>头像颜色</span><div class="color-options">${Object.entries(
      colors,
    )
      .map(
        ([c, n]) =>
          `<button type="button" class="color-option ${c} ${c === person.avatar_color ? "selected" : ""}" data-color="${c}" aria-label="${n}" aria-pressed="${c === person.avatar_color}"><span>${c === person.avatar_color ? "✓" : ""}</span></button>`,
      )
      .join(
        "",
      )}</div><small>姓名首字自动生成头像，暂不支持上传照片。</small></div><label class="field">想对校友说的话<span class="optional">选填</span><textarea name="message" maxlength="120" rows="3" placeholder="写一句近况，或一句想对老同学说的话…">${esc(person.message)}</textarea><small class="character-count"><span id="message-count">${person.message?.length || 0}</span> / 120</small></label>
    <label class="checkbox-field consent"><input name="consent" type="checkbox" required ${editing ? "checked" : ""}/><span>我已阅读<a href="/privacy" target="_blank">隐私与公开说明</a>，同意将所填基本资料公开展示。</span></label><p class="form-error" role="alert" tabindex="-1" hidden></p><button class="button primary full-width large" type="submit">${icon("card")}${editing ? "保存我的新近况" : "生成我的电子校友卡"}${icon("arrow")}</button><p class="form-submit-note">${editing ? "修改成功后，通讯录与大学去向同步更新。" : "提交后自动登录，可在个人中心更新资料。"}</p></form></section>
    <aside class="registration-aside"><div class="sticky-aside"><div class="preview-heading"><span class="eyebrow">YOUR ALUMNI ID</span><span>${icon("spark")}实时预览</span></div><div id="card-preview">${alumniCard(person, true)}</div><div class="aside-note"><span>一张卡片，一份归属</span><h3>从此山海，皆有同窗。</h3><p>这里记录的，不只是一份资料，<br>更是我们与母校始终相连的证明。</p><div>${icon("check")}基本资料可公开检索</div><div>${icon("lock")}联系方式默认受保护</div><div>${icon("card")}登录个人中心随时更新近况</div></div><div class="registration-quote">“聚是一团火，散是满天星。”<span>致每一位奔赴山海的榆中人</span></div></div></aside></div></div>`,
    bind(root) {
      const form = root.querySelector("#registration-form");
      const idempotencyKey = requestKey();
      const snapshot = () => {
        const d = Object.fromEntries(new FormData(form));
        d.graduation_year = Number(d.graduation_year) || "";
        d.contact_public = form.elements.contact_public.checked;
        return d;
      };
      const preview = () => {
        const p = snapshot();
        root.querySelector("#card-preview").innerHTML = alumniCard(p, true);
        root.querySelector("#message-count").textContent =
          form.elements.message.value.length;
      };
      form.addEventListener("input", preview);
      root.querySelectorAll("[data-color]").forEach(
        (b) =>
          (b.onclick = () => {
            form.elements.avatar_color.value = b.dataset.color;
            root.querySelectorAll("[data-color]").forEach((x) => {
              x.classList.toggle("selected", x === b);
              x.setAttribute("aria-pressed", String(x === b));
              x.querySelector("span").textContent = x === b ? "✓" : "";
            });
            preview();
          }),
      );
      form.elements.graduation_year.min = student
        ? new Date().getFullYear()
        : 1903;
      form.elements.graduation_year.max =
        new Date().getFullYear() + (student ? 6 : 0);
      root.querySelectorAll("[data-identity]").forEach(
        (button) =>
          (button.onclick = async () => {
            if (button.dataset.identity === form.elements.identity.value)
              return;
            const current = {
              ...person,
              ...snapshot(),
              identity: button.dataset.identity,
            };
            if (!editing)
              history.replaceState(
                {},
                "",
                `/register?identity=${current.identity}`,
              );
            const next = await register(
              new URL(location.href),
              initial,
              editCode,
              current,
            );
            root.innerHTML = next.html;
            next.bind(root);
          }),
      );
      form.onsubmit = (e) => {
        e.preventDefault();
        if (!form.reportValidity()) return;
        busy(form, async () => {
          const data = snapshot();
          data.graduation_year = Number(data.graduation_year);
          delete data.consent;
          const saved = await api(
            editing ? (editCode ? `/alumni/${person.id}` : "/me/alumni") : "/auth/register",
            {
              method: editing ? "PUT" : "POST",
              headers: { "Idempotency-Key": idempotencyKey },
              body: JSON.stringify({
                ...data,
                ...(editing ? { edit_code: editCode } : {}),
              }),
            },
          );
          if (saved.token) saveSession(saved);
          navigate(editing ? "/me" : `/alumni/${saved.id}`);
          toast(editing ? "近况已更新，祝你万事顺意。" : "校友卡已生成，欢迎回家。");
        });
      };
    },
  };
}

export async function profile(url) {
  const id = url.pathname.split("/").pop();
  const person = await api(`/alumni/${encodeURIComponent(id)}`);
  return {
    html: `<div class="wrap page-content profile-page"><a class="text-link back-link" href="/directory">${icon("back")}回到校友通讯录</a><div class="profile-layout"><section><div class="eyebrow">ONE OF US · FOREVER ALUMNI</div><h1>同窗如初，幸会。</h1>${alumniCard(person)}<div class="profile-actions"><button class="button outline" id="share-card">${icon("copy")}复制卡片链接</button><button class="button primary" id="edit-profile">${icon("card")}管理我的校友卡</button></div></section><section class="profile-info"><span class="eyebrow">THE STORY CONTINUES</span><h2>${esc(person.name)}的近况</h2><dl>${[
      ["身份", person.identity === "student" ? "在读同学" : "榆中校友"],
      ["届别", `${person.graduation_year}届`],
      ["班级", person.class_name],
      ["所在城市", person.city],
      ["专业", person.major],
      ["学历", person.education],
      ["职位", person.position],
      ["工作单位", person.company],
    ]
      .filter(([, v]) => v)
      .map(([k, v]) => `<div><dt>${k}</dt><dd>${esc(v)}</dd></div>`)
      .join(
        "",
      )}</dl>${person.message ? `<blockquote><span>想对校友说</span><p>“${esc(person.message)}”</p></blockquote>` : ""}<div class="contact-privacy">${icon("lock")}<div><strong>${person.contact_public ? "联系方式受隐私保护" : "本人未公开联系方式"}</strong><p>当前不向普通访问者展示手机号、微信和邮箱。<a href="/privacy">了解更多</a></p></div></div></section></div></div>`,
    bind(root) {
      root.querySelector("#share-card").onclick = async () => {
        const { copyText } = await import("./ui.js");
        copyText(location.href);
      };
      root.querySelector("#edit-profile").onclick = () => navigate(session() ? "/me" : "/login?next=/me");
    },
  };
}

export async function privacy() {
  return {
    html: `<article class="wrap page-content privacy-page">${sectionHead("PRIVACY & TRUST", "让重逢安心，让信任长久", "校友之家 · 隐私与公开说明（2026年11月11日）")}<div class="prose-panel"><h2>哪些内容会公开？</h2><p>登记后，你的姓名、届别、班级、所在城市、大学（在读同学的理想大学）、专业、学历、职位、工作单位与寄语将展示在公开通讯录和校友卡中。任何访客均可浏览、搜索，请仅填写你愿意公开的信息。</p><h2>联系方式如何保护？</h2><p>手机号、微信和邮箱属于受限信息，公开接口不会返回这些字段。当前尚未开放校友身份验证，因此所有普通访问者均无法查看联系方式。“公开联系方式”开关只表达你对未来已验证校友可见的意愿，登记本身不代表认证。邮箱默认始终不公开。</p><h2>如何修改或删除资料？</h2><p>领取校友卡时创建账号和密码，登录个人中心即可修改自己的资料与隐私选项。密码使用 bcrypt 哈希保存。旧版校友卡可在登录页凭校友卡编号和旧编辑码绑定新账号，绑定后旧编辑码失效。</p><p>照片上传成功后会获得删除码。你可以在照片的“管理照片”入口输入删除码并确认撤下。资料注销、编辑码遗失与内容异议可由网站维护者核验后通过管理接口处理；正式运营前将公布联系渠道，当前不收集额外身份材料。</p><h2>校园内容与审核</h2><p>照片、帖子和回复对所有人公开。请确保拥有图片分享权利，不发布他人的联系方式、身份证件或其他隐私。内容采用先发后审，维护者可下架违规内容并保留必要的操作审计。图片上传时会压缩、重新编码，移除原始图片元数据。</p><h2>浏览器、日志与演示模式</h2><p>浏览器仅保存登录 Token 与到期时间（7 天），退出后清除；服务端 Redis 仅保存 Token 哈希及会话。请勿在共享设备保留登录。网站不在浏览器持久保存密码、编辑码、删除码或联系方式。搜索条件保留在页面 URL 中，请不要在搜索框输入敏感信息。服务端会对公开写操作进行频率限制。演示模式下人物资料与社区内容为虚构，所有操作仅保存在服务进程内，重启即重置；演示模式不能用于正式个人资料登记。</p><p class="privacy-end">感谢你和我们一起，守护这份属于榆中人的共同记忆。</p><a href="/register" class="button primary">我已了解，去登记 ${icon("arrow")}</a></div></article>`,
  };
}

function nextPath(url) {
  const next = url.searchParams.get("next") || "/me";
  return next.startsWith("/") && !next.startsWith("//") && !next.includes("\\") ? next : "/me";
}
export async function login(url) {
  const claim = url.searchParams.get("claim") === "1";
  return {
    html: `<div class="wrap page-content auth-page">${sectionHead("WELCOME BACK", claim ? "绑定已有校友卡" : "欢迎回家，榆中人", claim ? "用原来的编辑码认领校友卡，今后使用账号登录。" : "登录后分享校园记忆、发帖交流和更新校友卡。")}
      <section class="form-panel"><form id="login-form">${field("账号", "account", {required:true,max:32,placeholder:claim ? "设置新账号" : "请输入账号"})}${field("密码", "password", {required:true,type:"password",max:72,placeholder:claim ? "设置至少 8 字节的密码" : "请输入密码"})}${claim ? `${field("旧校友卡编号", "alumni_id", {required:true,max:20,placeholder:"卡片上的 NO. 编号"})}${field("旧编辑码", "edit_code", {required:true,type:"password",max:80})}` : ""}<p class="form-error" role="alert" tabindex="-1" hidden></p><button class="button primary full-width" type="submit">${claim ? "绑定并登录" : "登录"}</button></form><p><a href="/register">首次来到这里？登记并领取校友卡</a></p><a href="${claim ? "/login" : "/login?claim=1"}" class="text-link">${claim ? "返回登录" : "已有旧校友卡？凭编辑码绑定账号"}</a></section></div>`,
    bind(root) {
      const form = root.querySelector("form");
      form.elements.account.autocomplete = "username";
      form.elements.password.autocomplete = claim ? "new-password" : "current-password";
      form.onsubmit = (event) => {
        event.preventDefault();
        busy(form, async () => {
          const data = Object.fromEntries(new FormData(form));
          if (claim) {
            data.alumni_id = Number(data.alumni_id);
            if (!Number.isSafeInteger(data.alumni_id) || data.alumni_id < 1) throw new Error("请输入有效的校友卡编号");
          }
          const result = await api(claim ? "/auth/register" : "/auth/login", {method:"POST", body:JSON.stringify(data)});
          saveSession(result);
          navigate(nextPath(url));
          toast("登录成功，欢迎回来。");
        });
      };
    },
  };
}
export async function personal(url) {
  if (!session()) return login(new URL("/login?next=/me", location.origin));
  let me;
  try { me = await api("/me"); }
  catch (error) { if (error.status === 401) return login(new URL("/login?next=/me", location.origin)); throw error; }
  const result = await register(url, me.profile);
  return {
    html: `<div class="wrap account-bar"><div><strong>个人中心</strong><span>当前账号：${esc(me.account)}</span></div><button class="button outline" id="logout">退出登录</button></div>${result.html}`,
    bind(root) {
      result.bind(root);
      root.querySelector("#logout").onclick = async () => {
        try { await api("/auth/logout", {method:"POST"}); saveSession(null); navigate("/login"); toast("已退出登录"); }
        catch (error) { toast(error.message); }
      };
    },
  };
}
