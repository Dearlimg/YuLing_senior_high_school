import { api } from "./api.js";
import { icon } from "./icons.js";
import {
  alumniCard,
  avatar,
  empty,
  esc,
  pagination,
  sectionHead,
} from "./ui.js";
import { navigate } from "./app.js";

const photoSource =
  "https://web.sjpt.ylrb.com/cms/rmt2420/2/2xw/yl/387623.shtml";
export async function home() {
  const [stats, alumni, photos] = await Promise.all([
    api("/stats"),
    api("/alumni?page_size=3"),
    api("/photos?page_size=3"),
  ]);
  const entries = [
    [
      "01",
      "people",
      "校友通讯录",
      "循着熟悉的名字，找回久违的同窗。",
      "/directory",
      "重逢同窗",
    ],
    [
      "02",
      "school",
      "大学去向",
      "从榆中出发，我们在更大的世界相遇。",
      "/universities",
      "看看大家去了哪里",
    ],
    [
      "03",
      "image",
      "校园映像",
      "珍藏一帧光影，留住一段共同的时光。",
      "/moments",
      "翻开校园相册",
    ],
    [
      "04",
      "chat",
      "校友贴吧",
      "近况、回忆、心愿，总有同窗愿意听。",
      "/forum",
      "坐下来聊聊",
    ],
  ];
  return {
    html: `<div class="wrap home-page">
    <section class="hero">
      <div class="hero-copy"><div class="eyebrow"><span class="eyebrow-line"></span>一所母校，一生牵挂 <span class="tiny-dot">·</span> SINCE 1903</div>
      <h1>山河万里，<br>我们仍是<span>榆中人。</span></h1>
      <p class="hero-description">从榆溪河畔出发，奔赴各自的山海。<br>领取我的校友卡，找到同班同学，看看榆中人都去了哪里。</p>
      <div class="hero-actions"><a class="button primary large" href="/register">领取我的校友卡 ${icon("arrow")}</a><a class="button text-button" href="/directory">寻找老同学 ${icon("up")}</a></div>
      <div class="hero-community"><div class="avatar-stack">${alumni.items.map((p) => avatar(p)).join("")}</div><span>${stats.alumni ? `<strong>${stats.alumni}</strong> 位同窗已在这里相聚` : "让我们从一次登记开始重逢"}<small>无论走多远，总有一份榆中情。</small></span></div></div>
      <div class="hero-visual"><div class="hero-image"><img src="/assets/campus-main.jpg" alt="榆林中学校园景致" fetchpriority="high" width="1000" height="680" /><div class="image-shade"></div><div class="hero-image-caption"><span>此心安处，亦是吾乡</span><small>YULIN HIGH SCHOOL · OUR SHARED HOME</small></div></div><div class="since-seal"><small>百年榆中</small><strong>1903</strong><small>薪火相传</small></div><div class="memory-note"><span class="note-pin"></span><span class="note-kicker">写给每一位榆中人</span><p>青春散场，<br>同窗<span>不散。</span></p><small>THE STORY CONTINUES ↗</small></div><span class="hero-side-label">OUR ROOTS. OUR STORIES. OUR HOME.</span></div>
    </section>
    <section class="stats-band" aria-label="校友之家数据">${[
      ["people", stats.alumni, "位校友与同学", "ALUMNI"],
      ["school", stats.universities, "所大学留下足迹", "UNIVERSITIES"],
      ["pin", stats.cities, "座城市彼此相连", "CITIES"],
      ["image", stats.photos, "帧校园共同记忆", "MEMORIES"],
    ]
      .map(
        ([i, n, t, en]) =>
          `<div class="stat-item">${icon(i)}<div><strong>${n}<span>+</span></strong><p>${t}</p></div><small>${en}</small></div>`,
      )
      .join("")}</section>
    <section class="home-section"><div class="section-heading"><div><span class="eyebrow">STAY CONNECTED</span><h2>把散落的故事，重新相连</h2></div><p>在这里，每一份近况都值得被看见。</p></div><div class="entry-grid">${entries.map(([n, i, t, d, url, cta]) => `<a href="${url}" class="entry-card"><div class="entry-top"><span class="entry-icon">${icon(i)}</span><small>${n}</small></div><h3>${t}</h3><p>${d}</p><span class="entry-link">${cta}${icon("arrow")}</span></a>`).join("")}</div></section>
    <section class="home-section reunion-section"><div class="reunion-main"><div class="section-heading"><div><span class="eyebrow">NICE TO SEE YOU AGAIN</span><h2>好久不见，老同学</h2></div><a href="/directory" class="text-link">全部校友 ${icon("arrow")}</a></div><div class="recent-alumni">${alumni.items.map(alumniCard).join("") || empty("第一份近况，等你书写", "登记资料，成为校友之家的一员。", '<a href="/register" class="text-link">去登记 ↗</a>')}</div></div><aside class="letter-card"><div class="eyebrow">A LETTER TO YOU</div>${icon("book")}<h3>走得再远，<br>记得常回家看看。</h3><p>写下你的名字和近况，<br>为校友之家的故事添上一页。</p><a href="/register">留下我的近况 ${icon("arrow")}</a><span class="letter-watermark">榆</span></aside></section>
    <section class="home-section"><div class="section-heading"><div><span class="eyebrow">MOMENTS WE SHARE</span><h2>有些时光，一直在心上</h2></div><a href="/moments" class="text-link">走进校园映像 ${icon("arrow")}</a></div><div class="home-photo-grid">${photos.items.length ? photos.items.map((p) => `<a href="/moments" class="home-photo"><img src="${esc(p.image_url)}" alt="${esc(p.caption)}" loading="lazy" width="600" height="400" /><div><small>${esc(p.category)}</small><h3>${esc(p.caption)}</h3><span>${esc(p.uploader)} 的校园记忆 ${icon("up")}</span></div></a>`).join("") : `<div class="home-photo empty-photo"><img src="/assets/campus-detail.jpg" alt="榆林中学校园" loading="lazy"/><div><small>校园光影</small><h3>让记忆，在这里安放</h3><a href="/moments" class="text-link">分享你的第一张照片 ↗</a></div></div>`}</div><p class="image-credit">首页校园摄影来源：<a href="${photoSource}" target="_blank" rel="noopener noreferrer">榆林日报 · 榆林中学 120 周年专题</a>。校友分享图片由上传者提供。</p></section>
    <section class="home-invitation"><div><span class="eyebrow">YOUR NEXT CHAPTER STARTS HERE</span><h2>世界很大，榆中是我们共同的起点。</h2></div><a class="button primary large" href="/register">加入校友之家 ${icon("arrow")}</a></section>
  </div>`,
  };
}

export async function directory(url) {
  const params = new URLSearchParams(url.search);
  const view = params.get("view") || "list";
  const page = Math.max(1, Number(params.get("page")) || 1);
  params.set("page_size", "12");
  params.set("page", page);
  const [result, meta] = await Promise.all([
    api(`/alumni?${params}`),
    api("/directory-meta"),
  ]);
  const grouped = view === "class" ? await api(`/classes?${params}`) : null;
  const year = params.get("graduation_year") || "";
  const keyword = params.get("keyword") || "";
  const uni = params.get("university") || "";
  const className = params.get("class_name") || "";
  const groups = grouped?.items || [];
  const classLink = (group) => {
    const next = new URLSearchParams(url.search);
    next.delete("page");
    next.delete("view");
    next.set("graduation_year", group.graduation_year);
    next.set("class_name", group.class_name || "__unassigned__");
    return `/directory?${next}`;
  };
  return {
    html: `<div class="wrap page-content">${sectionHead("OUR ALUMNI · RECONNECT", "校友通讯录", "熟悉的名字，不同的人生。无论身在何方，我们始终是同窗。", '<a href="/register" class="button primary">' + icon("plus") + "登记领卡</a>")}
    <a href="/register?identity=student" class="student-banner"><span class="student-banner-icon">${icon("school")}</span><div><strong>青春正当时，在读同学也来报到</strong><p>写下名字、班级和理想大学，提前与更大的榆中相遇。</p></div><span class="button light-button">去报到 ${icon("arrow")}</span></a>
    <section class="directory-panel"><div class="search-toolbar"><form id="directory-search" class="search-field">${icon("search")}<input name="keyword" aria-label="搜索校友" placeholder="搜姓名、大学、专业、城市、单位" value="${esc(keyword)}" maxlength="100" />${keyword ? '<button type="button" class="icon-button" id="clear-search" aria-label="清空搜索">' + icon("close") + "</button>" : ""}<button class="button primary" type="submit">搜索</button></form><div class="view-switch" aria-label="展示方式"><button data-view="list" class="${view === "list" ? "selected" : ""}" aria-pressed="${view === "list"}">${icon("list")}按列表</button><button data-view="class" class="${view === "class" ? "selected" : ""}" aria-pressed="${view === "class"}">${icon("grid")}按班级</button></div></div>
    <div class="year-filters" aria-label="届别筛选"><span>毕业届别</span>${["", ...meta.years.map(String)].map((y) => `<button data-year="${y}" class="chip ${year === y ? "active" : ""}" aria-pressed="${year === y}">${y ? y + "届" : "全部"}</button>`).join("")}</div>
    ${uni ? `<div class="active-filter">正在查看实际就读 <strong>${esc(uni)}</strong> 的校友 <button class="text-link" id="clear-university">清除条件 ×</button></div>` : ""}
    ${className ? `<div class="active-filter">当前班级 <strong>${esc(className === "__unassigned__" ? "未填写班级" : className)}</strong><button class="text-link" id="clear-class">清除班级 ×</button></div>` : ""}
    <div class="list-meta"><span>共 <strong>${meta.total}</strong> 位同学${keyword || year || uni ? ` <span class="subtle">/ 当前筛选 ${result.total} 位</span>` : ""}</span><a href="/universities" class="text-link">按大学看 ${icon("arrow")}</a></div>
    ${view === "class" ? (groups.length ? `<div class="class-grid">${groups.map((g) => `<a class="class-card" href="${esc(classLink(g))}"><span>${icon("people")}</span><small>${g.graduation_year} 届</small><h3>${esc(g.class_name || "未填写班级")}</h3><p>${g.count} 位同学 ${icon("arrow")}</p></a>`).join("")}</div>` : empty("还没有找到这个班级", "试试其他届别或关键词。", '<a href="/directory" class="button outline">清空筛选</a>')) : result.items.length ? `<div class="alumni-grid">${result.items.map(alumniCard).join("")}</div>` : empty("暂时没有找到这位同窗", "换个关键词，或邀请老同学来登记，让重逢更近一步。", '<a href="/directory" class="button outline">清空筛选</a>')}
    ${pagination(page, view === "class" ? grouped.total : result.total, 12)}</section><p class="privacy-note">${icon("lock")}通讯录基本资料公开可见，手机号、微信和邮箱不会向普通访问者展示。<a href="/privacy">了解隐私规则</a></p></div>`,
    bind(root) {
      const change = (key, value) => {
        const next = new URLSearchParams(url.search);
        if (value) next.set(key, value);
        else next.delete(key);
        next.delete("page");
        navigate(`/directory?${next}`);
      };
      root.querySelector("#directory-search").onsubmit = (e) => {
        e.preventDefault();
        change("keyword", new FormData(e.currentTarget).get("keyword").trim());
      };
      root
        .querySelector("#clear-search")
        ?.addEventListener("click", () => change("keyword", ""));
      root
        .querySelectorAll("[data-year]")
        .forEach(
          (b) => (b.onclick = () => change("graduation_year", b.dataset.year)),
        );
      root
        .querySelectorAll("[data-view]")
        .forEach((b) => (b.onclick = () => change("view", b.dataset.view)));
      root.querySelectorAll("[data-page]").forEach(
        (b) =>
          (b.onclick = () => {
            const q = new URLSearchParams(url.search);
            q.set("page", b.dataset.page);
            navigate(`/directory?${q}`);
          }),
      );
      root
        .querySelector("#clear-university")
        ?.addEventListener("click", () => change("university", ""));
      root
        .querySelector("#clear-class")
        ?.addEventListener("click", () => change("class_name", ""));
    },
  };
}

export async function universities(url) {
  const result = await api("/universities");
  const query = url.searchParams.get("keyword") || "";
  const filtered = result.items.filter((u) =>
    u.name.toLowerCase().includes(query.toLowerCase()),
  );
  const max = result.items[0]?.count || 1;
  return {
    html: `<div class="wrap page-content">${sectionHead("WHERE WE GO", "从榆中出发，去往更大的世界", "每一所大学，都有我们的故事。点开校名，遇见同路的学长学姐。")}
    <div class="university-layout"><aside class="university-aside"><div class="eyebrow">OUR FOOTPRINTS</div><h2>青春的下一站，<br><span>我们各自闪耀。</span></h2><div class="university-numbers"><div><strong>${result.items.length}</strong><span>所大学</span></div><div><strong>${result.total}</strong><span>位已填院校的校友</span></div></div><div class="university-art" aria-hidden="true">${icon("school")}<span>1903</span><p>怀天下 · 求真知 · 报国家</p></div><p class="subtle">仅统计校友实际就读院校，<br>在读同学的理想大学不计入榜单。</p><a href="/directory" class="text-link">去通讯录看看 ${icon("arrow")}</a></aside>
    <section class="university-ranking"><div class="ranking-title"><div><h2>大学去向名单</h2><small>以登记信息为据，留下每一步足迹。</small></div><span class="tag">按人数排序</span></div><form id="university-search" class="search-field">${icon("search")}<input name="keyword" placeholder="搜索大学名称" aria-label="搜索大学名称" value="${esc(query)}" maxlength="100"/><button class="button outline" type="submit">搜索</button></form>${filtered.length ? filtered.map((u) => `<a href="/directory?university=${encodeURIComponent(u.name)}" class="university-row"><span class="rank-number ${result.items.indexOf(u) < 3 ? "top-rank" : ""}">${String(result.items.indexOf(u) + 1).padStart(2, "0")}</span><div class="university-row-main"><div><h3>${esc(u.name)}</h3><span><strong>${u.count}</strong> 人 ${icon("up")}</span></div><div class="bar-track"><span style="width:${Math.max(3, (u.count / max) * 100)}%"></span></div></div></a>`).join("") : empty(query ? "没有找到这所大学" : "下一站，由你点亮", query ? "试试学校全称，或清空搜索。" : "填写实际就读院校，让你的大学出现在这里。", `<a href="${query ? "/universities" : "/register"}" class="button outline">${query ? "清空搜索" : "去登记"}</a>`)}<p class="ranking-footnote">相同人数按校名排序 · 常见学校简称自动归并</p></section></div></div>`,
    bind(root) {
      root.querySelector("#university-search").onsubmit = (e) => {
        e.preventDefault();
        navigate(
          `/universities?keyword=${encodeURIComponent(new FormData(e.currentTarget).get("keyword").trim())}`,
        );
      };
    },
  };
}
