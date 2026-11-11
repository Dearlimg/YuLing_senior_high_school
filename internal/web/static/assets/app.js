import { api } from "./api.js";
import { loading, errorState, closeModal } from "./ui.js";
import { home, directory, universities } from "./pages-discover.js";
import { register, profile, privacy } from "./pages-profile.js";
import { moments, forum } from "./pages-community.js";

const main = document.querySelector("#main");
const scrollPositions = new Map();
let routeKey = location.pathname + location.search;
let renderVersion = 0;
export function navigate(url, options = {}) {
  scrollPositions.set(routeKey, window.scrollY);
  if (options.replace) history.replaceState({}, "", url);
  else history.pushState({}, "", url);
  closeModal();
  render(options.restore);
}
window.addEventListener("popstate", () => {
  closeModal();
  render(true);
});
document.addEventListener("click", (event) => {
  const link = event.target.closest("a");
  if (
    link &&
    !event.ctrlKey &&
    !event.metaKey &&
    !event.shiftKey &&
    event.button === 0 &&
    !link.hasAttribute("download") &&
    link.target !== "_blank"
  ) {
    const url = new URL(link.href, location.origin);
    if (
      url.origin === location.origin &&
      !url.hash &&
      !url.pathname.startsWith("/api/")
    ) {
      event.preventDefault();
      navigate(url.pathname + url.search);
      return;
    }
  }
  const action = event.target.closest("[data-action]")?.dataset.action;
  if (action === "close-modal") closeModal();
  if (action === "retry") render();
});
document.querySelector("#modal").addEventListener("click", (event) => {
  if (event.target === event.currentTarget) closeModal();
});
document.addEventListener(
  "error",
  (event) => {
    if (event.target instanceof HTMLImageElement) {
      event.target.classList.add("image-failed");
      event.target.alt = `图片暂时无法加载 · ${event.target.alt}`;
    }
  },
  true,
);
export async function render(restore = false) {
  const version = ++renderVersion;
  routeKey = location.pathname + location.search;
  document.querySelectorAll("[data-nav]").forEach((link) => {
    const active = link.dataset.nav === location.pathname;
    link.classList.toggle("active", active);
    if (active) link.setAttribute("aria-current", "page");
    else link.removeAttribute("aria-current");
  });
  main.innerHTML = loading(location.pathname);
  main.className = "";
  const titles = {
    "/": "校友之家",
    "/directory": "校友通讯录",
    "/universities": "大学去向",
    "/register": "登记 · 领取校友卡",
    "/moments": "校园映像",
    "/forum": "校友贴吧",
    "/privacy": "隐私与公开说明",
  };
  document.title = `${titles[location.pathname] || "电子校友卡"} · 陕西省榆林中学`;
  const page =
    {
      "/": home,
      "/directory": directory,
      "/universities": universities,
      "/register": register,
      "/moments": moments,
      "/forum": forum,
      "/privacy": privacy,
    }[location.pathname] ||
    (location.pathname.startsWith("/alumni/") ? profile : null);
  try {
    const result = page
      ? await page(new URL(location.href))
      : { html: errorState(new Error("找不到这个页面")) };
    if (version !== renderVersion) return;
    main.innerHTML = result.html;
    result.bind?.(main);
    requestAnimationFrame(() =>
      window.scrollTo({
        top: restore ? scrollPositions.get(routeKey) || 0 : 0,
        behavior: "instant",
      }),
    );
  } catch (error) {
    if (version === renderVersion)
      main.innerHTML = `<div class="wrap">${errorState(error)}</div>`;
  }
}
api("/meta")
  .then((meta) => {
    if (meta.demo)
      document.querySelector("#mode-banner").innerHTML =
        '<div class="demo-banner">体验预览 · 人物与社区内容为演示数据，操作仅保存在当前服务进程，重启后重置。</div>';
  })
  .catch(() => {});
render();
