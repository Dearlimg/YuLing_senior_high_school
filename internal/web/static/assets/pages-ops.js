import { api } from "./api.js";
import { dateLabel, empty, esc, sectionHead } from "./ui.js";
import { render } from "./app.js";

// 隐藏运营看板：不挂导航与页脚，仅知道路由的管理员可访问。
// 密码即 ADMIN_TOKEN，只存模块内存，刷新后需重新输入（与 /admin bootstrap 同策略）。
let boardToken = "";

const boardAPI = (path) =>
  api(path, { headers: { Authorization: `Bearer ${boardToken}` } });

function gate(message = "") {
  return {
    html: `<div class="wrap page-content admin-gate">${sectionHead("SITE OPERATIONS", "访问看板")}<div class="composer-card"><p>${esc(message || "此页面仅面向站点维护者，输入管理密码后查看真实访客数据。")}</p><form id="ops-form"><label class="field">管理密码<input type="password" name="token" required minlength="32" autocomplete="off" placeholder="输入 ADMIN_TOKEN"/></label><p class="subtle">密码仅用于本次页面会话，不会保存；刷新后需重新输入。</p><p class="form-error" role="alert" tabindex="-1" hidden></p><button class="button primary" type="submit">查看看板</button></form></div></div>`,
    bind(root) {
      root.querySelector("#ops-form").onsubmit = (event) => {
        event.preventDefault();
        const form = event.currentTarget;
        const button = form.querySelector("button");
        button.disabled = true;
        boardToken = form.elements.token.value.trim();
        boardAPI("/admin/visitors")
          .then(() => render())
          .catch((error) => {
            boardToken = "";
            button.disabled = false;
            const box = form.querySelector(".form-error");
            box.textContent = error.status === 401 ? "密码不正确，请核对后再试" : error.message;
            box.hidden = false;
            box.focus();
          });
      };
    },
  };
}

function metricCard(label, value, sub = "") {
  return `<div class="composer-card" style="text-align:center"><p class="subtle">${label}</p><h2 style="margin:4px 0">${value}</h2>${sub ? `<small class="subtle">${sub}</small>` : ""}</div>`;
}

function trendChart(days) {
  if (!days.length) return empty("还没有访问数据", "有真实访客浏览页面后，这里会出现趋势。");
  const max = Math.max(...days.map((d) => d.pv), 1);
  return `<div style="display:flex;align-items:flex-end;gap:6px;height:140px" role="img" aria-label="近 ${days.length} 天 PV 趋势">${days
    .map(
      (d) => `<div style="flex:1;text-align:center" title="${esc(d.day)}：PV ${d.pv} / UV ${d.uv}">
        <div style="height:${Math.max(3, Math.round((d.pv / max) * 100))}px;background:var(--wine,#8c2f39);border-radius:4px 4px 0 0"></div>
        <small class="subtle" style="font-size:11px">${esc(d.day.slice(5))}</small>
        <br/><small class="subtle" style="font-size:11px">${d.uv}人</small>
      </div>`,
    )
    .join("")}</div>`;
}

function board(data) {
  const v = data.visits;
  const t = data.traffic || { available: false };
  const names = {
    "/": "首页", "/directory": "通讯录", "/universities": "大学去向", "/moments": "校园映像",
    "/forum": "校友贴吧", "/register": "登记领卡", "/events": "同窗重逢", "/login": "登录",
    "/me": "个人中心", "/class": "班级同学", "/updates": "同班动态", "/annual-report": "年度报告",
  };
  const trafficNote = t.available
    ? `今日全量请求 ${t.requests} 次、独立 IP ${t.ips} 个（含爬虫）；真实访客 UV ${v.today.uv}，机器流量占比约 ${t.ips > 0 ? Math.max(0, Math.round((1 - v.today.ips / t.ips) * 100)) : 0}%。`
    : "全量流量底账暂不可用（演示模式或 Redis 未就绪）。";
  return {
    html: `<div class="wrap page-content admin-page">
      ${sectionHead("SITE OPERATIONS", "访问看板", "真实访客 = 浏览器主动上报；扫描器不执行 JS，不计入 UV。", '<button class="button outline" id="ops-refresh">刷新数据</button>')}
      <div style="display:grid;grid-template-columns:repeat(auto-fit,minmax(140px,1fr));gap:12px">
        ${metricCard("今日 UV", v.today.uv, `独立 IP ${v.today.ips}`)}
        ${metricCard("今日 PV", v.today.pv, `昨日 ${v.yesterday.pv}`)}
        ${metricCard("昨日 UV", v.yesterday.uv, "")}
        ${metricCard("累计 PV", v.total.pv, `累计 UV ${v.total.uv}`)}
      </div>
      <p class="subtle">${trafficNote}</p>
      <h2>近 14 天趋势</h2>${trendChart(v.days)}
      <h2>近 7 天热门页面</h2>
      <div class="admin-rows">${v.pages.map((p) => `<article class="admin-row"><div><strong>${esc(names[p.path] || p.path)}</strong><p class="subtle">${esc(p.path)}</p></div><span>${p.pv} 次 · ${p.uv} 人</span></article>`).join("") || empty("暂无页面数据", "")}</div>
      <h2>最近访问明细</h2>
      <p class="subtle">仅展示脱敏信息：路径、来源、设备与访客标识前缀，不记录 IP 与 UA 原文。</p>
      <div class="admin-rows">${v.recent.map((r) => `<article class="admin-row"><div><strong>${esc(names[r.path] || r.path)}</strong><p class="subtle">${esc(r.path)} · ${esc(r.source)} · ${r.device === "mobile" ? "手机" : "桌面"} · 访客 ${esc(r.visitor_hash)}</p></div><small>${dateLabel(r.created_at)}</small></article>`).join("") || empty("暂无访问明细", "")}</div>
    </div>`,
    bind(root) {
      root.querySelector("#ops-refresh").onclick = () => render();
    },
  };
}

export async function opsBoard() {
  if (!boardToken) return gate();
  try {
    return board(await boardAPI("/admin/visitors"));
  } catch (error) {
    if ([401, 403].includes(error.status)) {
      boardToken = "";
      return gate(error.message);
    }
    throw error;
  }
}
