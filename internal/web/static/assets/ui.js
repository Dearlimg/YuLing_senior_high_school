import { icon } from "./icons.js";
export const esc = (value) =>
  String(value ?? "").replace(
    /[&<>"']/g,
    (char) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
        char
      ],
  );
export const colors = {
  red: "榆红",
  blue: "远山蓝",
  gold: "秋日金",
  pink: "海棠粉",
  green: "松柏绿",
};
export function avatar(person, cls = "") {
  return `<span class="avatar ${esc(person.avatar_color || "red")} ${cls}" aria-hidden="true">${esc(Array.from(person.name || person.nickname || "榆")[0])}</span>`;
}
export function alumniCard(person) {
  return `<a class="alumni-card" href="/alumni/${person.id}">${avatar(person)}<div class="alumni-info"><div class="name-line"><h3>${esc(person.name)}</h3><span class="year-tag">${person.graduation_year}届</span>${person.identity === "student" ? '<span class="student-tag">在读</span>' : ""}</div><p>${person.identity === "student" ? "理想大学 · " : ""}${esc([person.identity === "student" ? person.dream_university : person.university, person.major].filter(Boolean).join(" · ") || "院校信息未填写")}</p><small>${icon("pin")}${esc([person.city, person.company || person.class_name].filter(Boolean).join(" · ") || "期待与你相逢")}</small></div>${icon("up", "card-arrow")}</a>`;
}
export function sectionHead(kicker, title, description = "", right = "") {
  return `<div class="page-heading"><div><div class="eyebrow">${kicker}</div><h1>${title}</h1>${description ? `<p>${description}</p>` : ""}</div>${right}</div>`;
}
export function empty(title, description, action = "") {
  return `<div class="empty-state">${icon("book")}<h3>${title}</h3><p>${description}</p>${action}</div>`;
}
export function loading(path = "") {
  if (["/directory", "/moments", "/forum"].includes(path)) {
    return `<div class="wrap skeleton-page" role="status" aria-label="正在加载页面"><div class="skeleton skeleton-heading"></div><div class="skeleton skeleton-subtitle"></div><div class="skeleton-grid">${Array.from({ length: path === "/moments" ? 4 : 6 }, () => `<div class="skeleton ${path === "/moments" ? "skeleton-photo" : "skeleton-card"}"></div>`).join("")}</div><span class="sr-only">正在加载，请稍候</span></div>`;
  }
  return `<div class="loading-state" role="status"><span class="spinner"></span>正在翻开这一页…</div>`;
}
export function errorState(error) {
  return empty(
    "这一页暂时没有打开",
    esc(error.message),
    '<button class="button outline" data-action="retry">重新加载</button>',
  );
}
export function toast(message) {
  const el = document.querySelector("#toast");
  el.textContent = message;
  el.classList.add("visible");
  clearTimeout(toast.timer);
  toast.timer = setTimeout(() => el.classList.remove("visible"), 3400);
}
export function modal(title, body, cls = "") {
  const dialog = document.querySelector("#modal");
  document.querySelector("#modal-content").detailTicket = null;
  document.querySelector("#modal-content").innerHTML =
    `<div class="modal-head"><h2 id="modal-title">${title}</h2><button class="icon-button" data-action="close-modal" aria-label="关闭">${icon("close")}</button></div>${body}`;
  dialog.className = cls;
  if (!dialog.open) dialog.showModal();
}
export function closeModal() {
  document.querySelector("#modal-content").detailTicket = null;
  document.querySelector("#modal").close();
}
export function field(label, name, opts = {}) {
  const {
    required = false,
    value = "",
    placeholder = "",
    type = "text",
    max = 50,
    help = "",
  } = opts;
  return `<label class="field">${label}${required ? '<span class="required">*</span>' : '<span class="optional">选填</span>'}<input name="${name}" type="${type}" value="${esc(value)}" placeholder="${placeholder}" ${required ? "required" : ""} ${type === "number" ? 'min="1903" max="2032" step="1"' : `maxlength="${max}"`} autocomplete="${name === "name" ? "name" : name === "email" ? "email" : "off"}" />${help ? `<small>${help}</small>` : ""}</label>`;
}
export function formError(form, message) {
  const box = form.querySelector(".form-error");
  box.textContent = message;
  box.hidden = false;
  box.focus();
}
export async function busy(form, callback) {
  if (form.dataset.busy) return;
  form.dataset.busy = "true";
  const button = form.querySelector('[type="submit"]');
  const original = button.innerHTML;
  button.disabled = true;
  button.innerHTML = '<span class="spinner"></span> 正在提交…';
  form.querySelector(".form-error")?.setAttribute("hidden", "");
  try {
    await callback();
  } catch (err) {
    formError(form, err.message);
  } finally {
    delete form.dataset.busy;
    button.disabled = false;
    button.innerHTML = original;
  }
}
export function dateLabel(value) {
  const d = new Date(value);
  if (Number.isNaN(+d)) return "";
  const minutes = Math.max(0, Math.floor((Date.now() - d) / 60000));
  if (minutes < 1) return "刚刚";
  if (minutes < 60) return `${minutes} 分钟前`;
  if (minutes < 1440) return `${Math.floor(minutes / 60)} 小时前`;
  return new Intl.DateTimeFormat("zh-CN", {
    month: "long",
    day: "numeric",
    timeZone: "Asia/Shanghai",
  }).format(d);
}
export function pagination(page, total, pageSize) {
  const pages = Math.ceil(total / pageSize);
  if (pages <= 1) return "";
  return `<div class="pagination"><button class="button outline" data-page="${page - 1}" ${page <= 1 ? "disabled" : ""}>上一页</button><span>第 ${page} / ${pages} 页</span><button class="button outline" data-page="${page + 1}" ${page >= pages ? "disabled" : ""}>下一页</button></div>`;
}
export async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text);
    toast("已复制，请妥善保存");
  } catch {
    toast("浏览器无法自动复制，请长按或选中编辑码复制");
  }
}
export function secretDialog(title, code, explanation, link = "") {
  modal(
    title,
    `<div class="secret-success">${icon("check")}<p>${explanation}</p><div class="secret-code">${esc(code)}</div><button class="button outline" id="copy-secret">${icon("copy")}复制并妥善保存</button><p class="subtle">此码仅完整展示一次，关闭后无法找回。请勿分享给他人。</p>${link}</div>`,
  );
  document.querySelector("#copy-secret").onclick = () => copyText(code);
}
