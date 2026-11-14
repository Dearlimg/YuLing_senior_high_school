import { api, requestKey } from "./api.js";
import { contextFor } from "./growth.js";

// 真实访客 beacon：只有执行了 JS 的浏览器才会上报，扫描器天然被排除。
// 上报失败静默，绝不影响浏览；路径不含 query，避免带出敏感参数。
let entered = false;

export function pageview() {
  const path = location.pathname;
  if (path === "/ops-board") return; // 看板自身不计入，避免自污染
  let source = "internal";
  if (!entered) {
    source = "direct";
    if (document.referrer) {
      try {
        const host = new URL(document.referrer).hostname.toLowerCase().replace(/^www\./, "");
        if (host && host !== location.hostname) source = host;
      } catch { /* 非法 referer 按直接访问处理 */ }
    }
  }
  entered = true;
  api("/track/pageview", {
    method: "POST",
    keepalive: true,
    body: JSON.stringify({
      event_key: requestKey(),
      visitor_id: contextFor().visitor_id,
      path,
      source,
    }),
  }).catch(() => {});
}
