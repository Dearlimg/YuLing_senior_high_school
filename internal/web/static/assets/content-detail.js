import { api, requestKey, session } from "./api.js";
import { icon } from "./icons.js";
import { avatar, busy, dateLabel, esc, modal, toast } from "./ui.js";
import { navigate } from "./app.js";

export async function openPost(id) {
  modal("同窗动态", '<p class="subtle">正在打开这段近况…</p>', "content-dialog");
  const host = document.querySelector("#modal-content");
  const ticket = Symbol();
  host.detailTicket = ticket;
  try {
    const post = await api(`/posts/${id}`);
    if (host.detailTicket !== ticket || !document.querySelector("#modal").open) return;
    const loginPath = `/login?next=${encodeURIComponent(`/forum?post=${id}`)}`;
    host.querySelector(".modal-head h2").textContent = "同窗动态";
    host.insertAdjacentHTML("beforeend", `<div class="content-detail"><div class="detail-media">${post.image_url ? `<img src="${esc(post.image_url)}" alt="${esc(post.nickname)}分享的照片"/>` : `<div class="detail-text-cover"><span class="eyebrow">同窗手记</span><p>${esc(post.content)}</p><span>写给时光，也写给你</span></div>`}</div><section class="detail-panel"><header class="post-author">${avatar({name:post.nickname,avatar_color:post.avatar_color})}<div><strong>${esc(post.nickname)}</strong><small>${post.graduation_year ? `${post.graduation_year}届 · ` : ""}${dateLabel(post.created_at)}</small></div></header><div class="detail-scroll"><p class="detail-caption">${esc(post.content)}</p><h3 class="comments-heading">评论 <span data-comment-count>${post.reply_count}</span></h3><div class="detail-comments" aria-live="polite"></div><button class="text-link more-comments" hidden>查看更多评论</button><p class="comment-error form-error" role="alert" hidden></p></div><footer class="detail-actions"><button class="like-button" aria-pressed="false" aria-label="点赞">${icon("heart")}<span>${post.like_count}</span></button><span class="subtle">${icon("chat")} <span data-comment-total>${post.reply_count}</span></span><form class="detail-reply-form"><label class="sr-only" for="detail-reply">写下评论</label><textarea id="detail-reply" name="content" rows="2" maxlength="500" required placeholder="说点什么，让同窗听见…"></textarea><p class="form-error" role="alert" tabindex="-1" hidden></p><button class="button primary" type="submit">发送</button></form></footer></section></div>`);
    host.querySelector(":scope > .subtle")?.remove();
    const comments = host.querySelector(".detail-comments");
    const more = host.querySelector(".more-comments");
    let page = 1;
    const load = async (reset = false) => {
      more.disabled = true;
      const error = host.querySelector(".comment-error");
      error.hidden = true;
      try {
        const result = await api(`/posts/${id}/replies?page=${reset ? 1 : page}&page_size=20`);
        if (host.detailTicket !== ticket) return;
        if (reset) { page = 1; comments.innerHTML = ""; }
        comments.querySelector(".no-comments")?.remove();
        comments.insertAdjacentHTML("beforeend", result.items.map(r => `<article class="reply-item">${avatar({name:r.nickname},"small")}<div><strong>${esc(r.nickname)}</strong><p>${esc(r.content)}</p><small>${dateLabel(r.created_at)}</small></div></article>`).join("") || (!result.total ? '<p class="subtle no-comments">还没有评论，留下第一声问候吧。</p>' : ""));
        page++;
        more.hidden = !result.has_more;
        host.querySelectorAll("[data-comment-count], [data-comment-total]").forEach(el => el.textContent = result.total);
        document.querySelectorAll(`[data-post-count="${id}"]`).forEach(el => el.textContent = result.total);
      } catch (err) { error.textContent = err.message; error.hidden = false; more.hidden = false; }
      finally { more.disabled = false; }
    };
    more.onclick = () => load();
    const form = host.querySelector("form");
    let key = requestKey();
    form.onsubmit = event => {
      event.preventDefault();
      if (!session()) { navigate(loginPath); return; }
      busy(form, async () => {
        await api(`/posts/${id}/replies`, {method:"POST", headers:{"Idempotency-Key":key}, body:JSON.stringify({content:form.elements.content.value})});
        key = requestKey(); form.reset(); await load(true); toast("评论已发送");
      });
    };
    const like = host.querySelector(".like-button");
    let liked = false;
    const paint = state => {
      liked = state.liked; like.setAttribute("aria-pressed", String(liked));
      like.setAttribute("aria-label", liked ? "取消点赞" : "点赞");
      like.querySelector("span").textContent = state.like_count;
      document.querySelectorAll(`[data-post-likes="${id}"]`).forEach(el => el.textContent = state.like_count);
    };
    like.disabled = !!session();
    like.onclick = async () => {
      if (!session()) { navigate(loginPath); return; }
      like.disabled = true;
      try { paint(await api(`/posts/${id}/likes`, {method:"PUT",body:JSON.stringify({liked:!liked})})); }
      catch(err) { toast(err.message); }
      finally { like.disabled = false; }
    };
    if (session()) api(`/posts/${id}/likes`).then(paint).catch(err => toast(err.message)).finally(() => {like.disabled = false;});
    await load();
  } catch (err) {
    if (host.detailTicket === ticket) {
      host.querySelector(":scope > .subtle").textContent = err.message;
      host.insertAdjacentHTML("beforeend", '<button class="button outline detail-retry">重试</button>');
      host.querySelector(".detail-retry").onclick = () => openPost(id);
    }
  }
}
