import { api, session } from "./api.js";
import { navigate, render } from "./app.js";
import { contentShareURL, shareContext, track } from "./growth.js";
import { shareCanvas, shareFooter } from "./media-sharing.js";
import { lines, presentPoster } from "./poster.js";
import { copyText, empty, esc, field, pagination, sectionHead, toast } from "./ui.js";

const themes = [
  {id:"anniversary",kicker:"A SHARED ROOT",title:"百年榆中，等你回家",description:"把母校的故事续写下去，从同班同学的一声报到开始。",action:"为母校留下我的名字",image:"campus-main.jpg"},
  {id:"graduation",kicker:"THE STORY CONTINUES",title:"毕业有期，同窗无期",description:"去更大的世界之前，留下名字与心愿；多年以后，还能循着这张卡找到彼此。",action:"留下我们的毕业记忆",image:"campus-detail.jpg"},
  {id:"homecoming",kicker:"HOME IS WHERE WE MEET",title:"春节返乡，再见老同学",description:"走得再远，也有熟悉的人和故事。邀请同班同学，先在这里说一声好久不见。",action:"邀请同班同学重逢",image:"campus-main.jpg"},
];

function topicParams(url,source,campaign) {
  const params=new URLSearchParams({source,campaign});
  const share=url.searchParams.get("share_id");
  if(/^[a-zA-Z0-9_-]{1,48}$/.test(share||""))params.set("share_id",share);
  return params;
}

export function eventEntriesHTML() {
  return `<section class="home-section"><div class="section-heading"><div><span class="eyebrow">ALWAYS A REASON TO REUNITE</span><h2>把重逢，写进下一页</h2></div><a class="text-link" href="/annual-report">翻看年度校友报告 →</a></div><div class="event-grid">${themes.map(t=>`<a class="event-card" href="/events?${new URLSearchParams({theme:t.id,source:"event_topic",campaign:t.id})}"><span class="eyebrow">${t.kicker}</span><h3>${t.title}</h3><p>${t.description}</p><span class="text-link">${t.action} →</span></a>`).join("")}</div></section>`;
}

export async function events(url) {
  const theme = themes.find(t=>t.id===url.searchParams.get("theme")) || themes[0];
  const stats = await api("/stats");
  const year = Number(url.searchParams.get("graduation_year")) || "";
  const name = (url.searchParams.get("class_name") || "").slice(0,30);
  const registration=topicParams(url,"event_topic",theme.id);
  return {
    html:`<div class="wrap page-content event-page"><div class="topic-tabs">${themes.map(t=>`<a class="chip ${t.id===theme.id?"active":""}" ${t.id===theme.id?'aria-current="page"':""} href="/events?${new URLSearchParams({theme:t.id,source:"event_topic",campaign:t.id})}">${t.id==="anniversary"?"校庆报到":t.id==="graduation"?"毕业季":"春节返乡"}</a>`).join("")}</div><section class="event-hero"><div><span class="eyebrow">${theme.kicker}</span><h1>${theme.title}</h1><p>${theme.description}</p><p class="event-count"><strong>${stats.alumni}</strong> 位校友与同学已在这里相聚</p><div class="growth-actions"><a class="button primary" href="/register?${registration}">${theme.action} →</a><button class="button outline" id="share-event">保存主题邀请海报</button><button class="button outline" id="copy-event">复制专题链接</button></div></div><img src="/assets/${theme.image}" alt="榆林中学校园" width="720" height="500"/></section><p class="subtle">校友自发维护的长期报到主题，非学校官方活动通知；不涉及线下活动安排或报名收费。</p><section class="form-panel event-class-form"><h2>这次，和同班同学一起报到</h2><p class="subtle">填写届别与班级，直接进入对应班级；尚未毕业也可以填写预计毕业年份。</p><form id="event-class"><div class="form-grid">${field("毕业年份","graduation_year",{type:"number",required:true,value:year,placeholder:"如 2018"})}${field("班级","class_name",{required:true,max:30,value:name,placeholder:"如 高三1班"})}</div><button type="submit" class="button primary">进入我的班级报到页 →</button></form></section>${eventEntriesHTML()}<div class="growth-actions"><a href="/moments" class="text-link">分享一张校园记忆 →</a><a href="/updates" class="text-link">查看订阅班级动态 →</a></div></div>`,
    bind(root) {
      const form=root.querySelector("#event-class");
      form.elements.graduation_year.max=new Date().getFullYear()+6;
      form.onsubmit=e=>{
        e.preventDefault();
        const params=new URLSearchParams(new FormData(form));
        params.set("source","event_topic");params.set("campaign",theme.id);
        const share=url.searchParams.get("share_id");
        if(/^[a-zA-Z0-9_-]{1,48}$/.test(share||"")) params.set("share_id",share);
        navigate(`/class?${params}`);
      };
      root.querySelector("#copy-event").onclick=()=>{
        const link=contentShareURL("/events",{theme:theme.id},"event_topic",theme.id);
        copyText(`${theme.title}\n${link.href}`);track("share_click",{},shareContext(link));
      };
      root.querySelector("#share-event").onclick=()=>{
        try {
          const {canvas,ctx}=shareCanvas(theme.kicker);
          ctx.fillStyle="#741f31";ctx.font='bold 54px "Microsoft YaHei",sans-serif';lines(ctx,theme.title,75,310,750,80,2);
          ctx.fillStyle="#78634c";ctx.font='29px "Microsoft YaHei",sans-serif';lines(ctx,theme.description,75,510,750,54,3);
          ctx.fillStyle="#741f31";ctx.font='bold 48px Georgia,serif';ctx.fillText(`${stats.alumni} 位同窗`,75,770);
          ctx.font='23px "Microsoft YaHei",sans-serif';ctx.fillText(`已报到 · 截至 ${new Date().toLocaleDateString("zh-CN")}`,75,820);
          ctx.fillStyle="#78634c";ctx.font='20px "Microsoft YaHei",sans-serif';ctx.fillText("校友自发报到主题 · 非学校官方活动通知",75,915);
          const link=contentShareURL("/events",{theme:theme.id},"event_topic",theme.id);
          shareFooter(ctx,link,"扫码领取校友卡，和同班同学说声好久不见。");
          presentPoster(canvas,{url:link,title:theme.title});
        } catch {toast("主题海报生成失败，请重试。");}
      };
    },
  };
}

export async function annualReport(url) {
  const year = url.searchParams.get("year") || new Date().getFullYear();
  const report = await api(`/annual-report?${new URLSearchParams({year})}`);
  const metrics = [[report.registrations,"位新报到同窗"],[report.graduates,"位毕业校友"],[report.students,"位在读同学"],[report.classes,"个班级新增报到"],[report.cities,"座城市留下足迹"],[report.photos,"张新校园记忆"],[report.posts,"篇新同窗手记"],[report.universities.length,"所实际就读院校"]];
  const cutoff = new Date(report.as_of).toLocaleString("zh-CN",{timeZone:"Asia/Shanghai"});
  const registration=topicParams(url,"annual_report",`annual_${report.year}`);
  return {
    html:`<div class="wrap page-content annual-page">${sectionHead("OUR YEAR TOGETHER",`${report.year}，我们在这里重逢`,"每一个真实的名字、每一段珍藏的记忆，都在续写榆中的故事。")}
    <div class="annual-tools"><form id="report-year">${field("报告年份","year",{required:true,type:"number",value:report.year})}<button class="button outline" type="submit">查看报告</button></form><button class="button primary" id="share-report">保存年度报告海报</button></div><section class="report-stats">${metrics.map(([n,label])=>`<article><strong>${n}</strong><span>${label}</span></article>`).join("")}</section>
    <p class="report-method">统计区间：${report.year} 年 1 月 1 日至 ${esc(cutoff)}（北京时间，结束时刻不含）。按该年新创建且当前仍公开的有效档案、照片和帖子统计；城市、班级、院校取这些新档案的当前填写值，不代表全体校友或历史快照。实际院校仅含毕业校友，不含在读同学的理想大学。当前年度持续更新。</p>
    <div class="annual-columns"><section class="form-panel"><h2>${report.year} 年，有哪些班级加入了故事？</h2><p class="subtle">按当年新登记人数展示前 10 个班级</p><ol class="report-list">${report.top_classes.map(c=>`<li><a href="/class?${new URLSearchParams({graduation_year:c.graduation_year,class_name:c.class_name})}">${c.graduation_year} 届 ${esc(c.class_name)}</a><strong>${c.count} 人</strong></li>`).join("")||"<li>还没有公开的班级报到记录</li>"}</ol></section><section class="form-panel"><h2>${report.year} 年报到的校友，从榆中走向哪里？</h2><p class="subtle">实际院校 · 前 10 所</p><ol class="report-list">${report.universities.slice(0,10).map(u=>`<li><span>${esc(u.name)}</span><strong>${u.count} 人</strong></li>`).join("")||"<li>还没有实际就读院校记录</li>"}</ol></section></div><div class="growth-actions"><a class="button primary" href="/register?${registration}">让下一次重逢，有我的名字 →</a><a class="text-link" href="/events">邀请老同学回家 →</a></div></div>`,
    bind(root) {
      const form=root.querySelector("#report-year");form.elements.year.max=new Date().getFullYear();
      form.onsubmit=e=>{e.preventDefault();navigate(`/annual-report?year=${form.elements.year.value}`);};
      root.querySelector("#share-report").onclick=()=>{
        try {
          const {canvas,ctx}=shareCanvas("OUR YEAR TOGETHER · 年度校友报告");
          ctx.fillStyle="#741f31";ctx.font='bold 52px "Microsoft YaHei",sans-serif';ctx.fillText(`${report.year}，我们在这里重逢`,75,285);
          metrics.slice(0,6).forEach(([n,label],i)=>{
            const x=75+(i%2)*390,y=400+Math.floor(i/2)*160;
            ctx.fillStyle="#741f31";ctx.font='bold 65px Georgia,serif';ctx.fillText(String(n),x,y);
            ctx.fillStyle="#78634c";ctx.font='24px "Microsoft YaHei",sans-serif';ctx.fillText(label,x,y+46);
          });
          ctx.font='20px "Microsoft YaHei",sans-serif';lines(ctx,`统计当年新增且当前公开的内容 · 截至 ${cutoff}（北京时间）\n自主登记数据，不代表全体校友。扫码查看完整统计口径。`,75,883,750,30,2);
          const link=contentShareURL("/annual-report",{year:report.year},"annual_report",`annual_${report.year}`);
          shareFooter(ctx,link,"每一次报到，都让我们离老同学更近一点。");
          presentPoster(canvas,{url:link,title:`${report.year}年度校友报告`});
        } catch {toast("年度报告海报生成失败，请重试。");}
      };
    },
  };
}

export async function updates(url) {
  if(!session()) return {html:`<div class="wrap page-content">${empty("登录后，接收同班同学的新故事","订阅自己关心的班级，只在站内查看公开动态。",'<a class="button primary" href="/login?next=/updates">登录并查看</a>')}</div>`};
  const page=Math.max(1,Number(url.searchParams.get("page"))||1);
  const [subscriptions,feed]=await Promise.all([api("/me/subscriptions"),api(`/me/updates?page=${page}&page_size=20`)]);
  const target=row=>row.kind==="alumni"?`/alumni/${row.target_id}`:`/${row.kind==="photo"?"moments?photo":"forum?post"}=${row.target_id}`;
  const actions={alumni:"报到 / 更新近况",post:"分享了同窗手记",photo:"上传了校园记忆"};
  return {
    html:`<div class="wrap page-content updates-page">${sectionHead("A LITTLE CLOSER TO CLASSMATES","同班的新故事",`当前 ${feed.unread} 条未读公开动态。订阅后开始收录，随时可以取消。`,'<button class="button outline" id="refresh-updates">刷新动态</button>')}
    <div class="updates-layout"><aside class="form-panel subscriptions-panel"><h2>我的班级订阅 · ${subscriptions.items.length}/20</h2>${subscriptions.items.map((s,i)=>`<div class="subscription-row"><a href="/class?${new URLSearchParams({graduation_year:s.graduation_year,class_name:s.class_name})}">${s.graduation_year} 届 ${esc(s.class_name)}</a><button class="text-link" data-unsubscribe="${i}">取消订阅</button></div>`).join("")||'<p class="subtle">还没有订阅班级。到班级页点击“订阅这个班的新故事”。</p>'}<a class="text-link" href="/directory?view=class">去找我的班级 →</a><p class="subtle">仅提供站内动态，不发送短信或邮件。动态取自当前公开资料；隐藏或删除后将不再展示，档案近况只保留最新一次。</p></aside><section><div class="updates-toolbar"><span>${feed.total} 条动态</span><button class="button outline" id="read-updates" ${feed.unread?"":"disabled"}>将本次刷新前动态全部标为已读</button></div><div class="activity-list">${feed.items.map(row=>`<a class="activity-row ${row.unread?"unread":""}" href="${target(row)}"><div><span class="activity-dot" aria-label="${row.unread?"未读":"已读"}"></span><strong>${esc(row.name)}</strong><span>${actions[row.kind]}</span></div><p>${esc(row.summary)||"欢迎回家，同窗。"}</p><small>${row.graduation_year} 届 ${esc(row.class_name)} · ${new Date(row.occurred_at).toLocaleString("zh-CN")}</small></a>`).join("")||empty("还没有新动态","订阅后的公开报到、近况、帖子和照片会在这里出现。")}</div>${pagination(page,feed.total,20)}</section></div></div>`,
    bind(root) {
      root.querySelector("#refresh-updates").onclick=()=>render();
      root.querySelector("#read-updates").onclick=async e=>{
        e.currentTarget.disabled=true;
        try {await api("/me/updates/read",{method:"PUT",body:JSON.stringify({through:feed.as_of})});await render();toast("本次刷新前的动态已标为已读。");}
        catch(error){if(e.target.isConnected)e.target.disabled=false;toast(error.message);}
      };
      root.querySelectorAll("[data-unsubscribe]").forEach(button=>button.onclick=async()=>{
        button.disabled=true;
        const s=subscriptions.items[Number(button.dataset.unsubscribe)];
        try {await api("/me/subscriptions",{method:"PUT",body:JSON.stringify({...s,subscribed:false})});await render();toast("已取消订阅，重新订阅将从当时开始收录。");}
        catch(error){button.disabled=false;toast(error.message);}
      });
      root.querySelectorAll("[data-page]").forEach(b=>b.onclick=()=>navigate(`/updates?page=${b.dataset.page}`));
    },
  };
}

export function bindClassSubscription(root,person) {
  const button=root.querySelector("#subscribe-class");
  if(!button) return;
  let subscribed=false;
  const setLabel=()=>{button.textContent=subscribed?"已订阅 · 取消订阅":"订阅这个班的新故事";button.setAttribute("aria-pressed",String(subscribed));};
  if(session()) {
    button.disabled=true;
    api("/me/subscriptions").then(data=>{
      subscribed=data.items.some(s=>s.graduation_year===person.graduation_year&&s.class_name===person.class_name);setLabel();
    }).catch(error=>toast(error.message)).finally(()=>button.disabled=false);
  }
  button.onclick=async()=>{
    if(!session()){navigate(`/login?next=${encodeURIComponent(location.pathname+location.search)}`);return;}
    button.disabled=true;
    try {await api("/me/subscriptions",{method:"PUT",body:JSON.stringify({...person,subscribed:!subscribed})});subscribed=!subscribed;setLabel();toast(subscribed?"已订阅；之后的新故事可在站内动态中查看。":"已取消这个班级的订阅。");}
    catch(error){toast(error.message);}finally{button.disabled=false;}
  };
}
