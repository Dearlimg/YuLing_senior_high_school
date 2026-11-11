import { api, compressImage, requestKey, session } from "./api.js";
import { icon } from "./icons.js";
import {
  avatar,
  busy,
  closeModal,
  dateLabel,
  empty,
  esc,
  field,
  modal,
  pagination,
  sectionHead,
  secretDialog,
  toast,
} from "./ui.js";
import { navigate, render } from "./app.js";

const pickedImages = new WeakMap();
const sizeLabel = bytes => bytes < 1024 * 1024 ? `${(bytes / 1024).toFixed(0)}KB` : `${(bytes / 1024 / 1024).toFixed(2)}MB`;
function imagePicker(required = false) {
  return `<div class="image-picker"><label class="upload-zone"><input name="image" type="file" accept="image/jpeg,image/png" ${required ? "required" : ""}/><span class="upload-prompt">${icon("upload")}<strong>点这里选一张照片</strong><small>JPG / PNG · 上传最多 2MB · 入库最多 512KB</small></span><img class="upload-preview" alt="所选照片预览" hidden/><span class="upload-filename"></span></label><div class="image-tools" hidden><p class="image-size" role="status"></p><button class="button outline slim-image" type="button">${icon("spark")}一键瘦身</button><a class="text-link slim-download" hidden download>下载瘦身图片</a></div><small class="subtle">瘦身在本机完成，支持 25MB / 4000 万像素以内原图，长边最多 2048 像素。合规图片上传时也会压缩并移除元数据。</small></div>`;
}
function bindPicker(form) {
  const input = form.elements.image;
  if (!input) return;
  const picker = input.closest(".image-picker");
  const zone = picker.querySelector(".upload-zone");
  const preview = picker.querySelector("img");
  const filename = picker.querySelector(".upload-filename");
  const tools = picker.querySelector(".image-tools");
  const size = picker.querySelector(".image-size");
  const button = picker.querySelector(".slim-image");
  const download = picker.querySelector(".slim-download");
  let version = 0, objectURL;
  const show = file => {
    if (objectURL) URL.revokeObjectURL(objectURL);
    objectURL = URL.createObjectURL(file);
    preview.src = objectURL;
    preview.hidden = false;
    zone.classList.add("has-image");
    filename.textContent = `${file.name} · ${sizeLabel(file.size)} · 点击可重新选择`;
  };
  input.onchange = () => {
    version++;
    const file = input.files[0];
    pickedImages.delete(form);
    tools.hidden = true;
    download.hidden = true;
    preview.hidden = true;
    zone.classList.remove("has-image");
    filename.textContent = "";
    button.disabled = false;
    if (objectURL) { URL.revokeObjectURL(objectURL); objectURL = null; }
    if (!file) return;
    if (!["image/jpeg", "image/png"].includes(file.type) || file.size > 25 * 1024 * 1024) {
      input.value = "";
      toast("请选择 25MB 以内的 JPG 或 PNG 图片，可先使用其他工具裁剪");
      return;
    }
    show(file);
    tools.hidden = false;
    size.textContent = `原图 ${sizeLabel(file.size)}。${file.size > 2 * 1024 * 1024 ? "超过上传限制，请先点击一键瘦身。" : "可直接上传，也可先瘦身预览。"}`;
  };
  button.onclick = async () => {
    const file = input.files[0];
    if (!file || button.disabled) return;
    const current = version;
    button.disabled = true;
    size.textContent = "正在本机瘦身，请稍候…";
    try {
      const slim = await compressImage(file);
      if (current !== version) return;
      pickedImages.set(form,slim);
      form.querySelector(".form-error")?.setAttribute("hidden", "");
      show(slim);
      size.textContent = `瘦身完成：${sizeLabel(file.size)} → ${sizeLabel(slim.size)}，减少 ${Math.max(0,Math.round((1-slim.size/file.size)*100))}%，已符合上传规范。`;
      download.href = objectURL;
      download.download = slim.name;
      download.hidden = false;
    } catch (error) { if (current === version) size.textContent = error.message; }
    finally { if (current === version) button.disabled = false; }
  };
}
async function selectedImage(form) {
  const prepared = pickedImages.get(form);
  if (prepared) return prepared;
  const file = form.elements.image.files[0];
  if (file?.size > 2 * 1024 * 1024) throw new Error("图片超过 2MB，请先点击一键瘦身，再提交");
  return compressImage(file);
}

export async function moments(url) {
  const page = Math.max(1, Number(url.searchParams.get("page")) || 1);
  const category = url.searchParams.get("category") || "";
  const result = await api(
    `/photos?page=${page}&page_size=12&category=${encodeURIComponent(category)}`,
  );
  const categories = ["校园风景", "老照片", "运动场上", "其他"];
  return {
    html: `<div class="wrap page-content">${sectionHead("CAMPUS MOMENTS", "把时光，留在校园里", "教学楼前的合影、运动会的冲刺、毕业那天的教室。总有一帧画面，让我们回到从前。", '<button class="button primary" id="upload-photo">' + icon("plus") + "分享一张照片</button>")}<div class="moments-intro"><div><span class="eyebrow">OUR SHARED MEMORIES</span><h2>记忆会泛黄，<br>而青春始终明亮。</h2><p>所有榆中人共享的照片墙，<br>让散落的光影，汇成我们共同的相册。</p></div><img src="/assets/campus-detail.jpg" alt="榆林中学校园历史影像" width="720" height="320"/><span class="moments-intro-label">岁月留声 · 榆中记忆</span></div><div class="section-heading album-heading"><div><h2>共享相册 <span class="count-label">${result.total} 张照片</span></h2></div><div class="category-filters">${["", ...categories].map((c) => `<button class="chip ${category === c ? "active" : ""}" data-category="${c}" aria-pressed="${category === c}">${c || "全部记忆"}</button>`).join("")}</div></div>${result.items.length ? `<div class="photo-columns">${result.items.map((p) => `<article class="photo-card"><button class="photo-open" data-image="${esc(p.image_url)}" data-caption="${esc(p.caption)}" aria-label="预览：${esc(p.caption)}"><img src="${esc(p.image_url)}" alt="${esc(p.caption)}" loading="lazy"/></button><div class="photo-card-info"><span class="photo-category">${esc(p.category)}</span><h3>${esc(p.caption)}</h3><div><span>${esc(p.uploader)} <i>·</i> ${dateLabel(p.created_at)}</span><button class="text-link manage-photo" data-id="${p.id}">管理照片</button></div></div></article>`).join("")}</div>` : empty("这里，等一张你的校园记忆", "第一张照片，也许就是故事的开始。", '<button class="button primary" id="empty-upload">分享照片</button>')}${pagination(page, result.total, 12)}<p class="privacy-note">${icon("image")}照片对所有人可见，先发后审。请勿上传未获许可的照片或他人隐私。登录后可以撤下自己上传的照片。</p><p class="image-credit">校园历史影像来源：<a href="https://web.sjpt.ylrb.com/cms/rmt2420/2/2xw/yl/387623.shtml" target="_blank" rel="noopener noreferrer">榆林日报 · 榆林中学 120 周年专题</a></p></div>`,
    bind(root) {
      const openUpload = () => {
        if (!session()) { navigate("/login?next=/moments"); return; }
        modal(
          "分享一帧校园记忆",
          `<p class="modal-description">让回忆有个归处。上传内容对所有人可见，先发后审。</p><form id="photo-form">${imagePicker(true)}${field("照片说明", "caption", { required: true, placeholder: "为这段时光写一句话", max: 80 })}<div class="form-grid"><p class="subtle">上传人使用你的校友卡姓名</p><label class="field">照片分类<select name="category">${categories.map((c) => `<option>${c}</option>`).join("")}</select></label></div><p class="form-error" role="alert" tabindex="-1" hidden></p><button class="button primary full-width" type="submit">${icon("upload")}上传到校园映像</button><small class="form-submit-note">上传后可使用当前账号撤下自己的照片。</small></form>`,
        );
        const f = document.querySelector("#photo-form");
        bindPicker(f);
        const key = requestKey();
        f.onsubmit = (e) => {
          e.preventDefault();
          busy(f, async () => {
            const data = new FormData(f);
            data.set("image", await selectedImage(f));
            const saved = await api("/photos", {
              method: "POST",
              headers: { "Idempotency-Key": key },
              body: data,
            });
            await render();
            toast("照片已上传，可使用当前账号管理。");
          });
        };
      };
      root.querySelector("#upload-photo").onclick = openUpload;
      root
        .querySelector("#empty-upload")
        ?.addEventListener("click", openUpload);
      root
        .querySelectorAll("[data-category]")
        .forEach(
          (b) =>
            (b.onclick = () =>
              navigate(
                `/moments?category=${encodeURIComponent(b.dataset.category)}`,
              )),
        );
      root
        .querySelectorAll("[data-page]")
        .forEach(
          (b) =>
            (b.onclick = () =>
              navigate(
                `/moments?page=${b.dataset.page}&category=${encodeURIComponent(category)}`,
              )),
        );
      root
        .querySelectorAll("[data-image]")
        .forEach(
          (b) =>
            (b.onclick = () =>
              modal(
                "校园记忆",
                `<img class="lightbox-image" src="${esc(b.dataset.image)}" alt="${esc(b.dataset.caption)}"/><p class="lightbox-caption">${esc(b.dataset.caption)}</p>`,
                "lightbox",
              )),
        );
      root.querySelectorAll(".manage-photo").forEach(
        (b) =>
          (b.onclick = () => {
            if (!session()) { navigate("/login?next=/moments"); return; }
            modal(
              "撤下我的照片",
              `<p class="modal-description">登录后可撤下自己上传的照片；旧照片还需提供上传时保存的删除码。撤下后将不再公开展示。</p><form id="delete-photo-form">${field("删除码（仅旧照片需要）", "delete_code", { required: false, type: "password", max: 80, placeholder: "请输入这张照片的删除码" })}<label class="checkbox-field"><input type="checkbox" required/><span>我确认要撤下这张照片</span></label><p class="form-error" role="alert" tabindex="-1" hidden></p><button class="button primary full-width" type="submit">确认撤下</button></form>`,
            );
            document.querySelector("#delete-photo-form").onsubmit = (e) => {
              e.preventDefault();
              const f = e.currentTarget;
              busy(f, async () => {
                await api(`/photos/${b.dataset.id}`, {
                  method: "DELETE",
                  body: JSON.stringify({
                    delete_code: new FormData(f).get("delete_code"),
                  }),
                });
                closeModal();
                render();
                toast("照片已撤下，记忆依然珍藏。");
              });
            };
          }),
      );
    },
  };
}

function postCard(post) {
  return `<article class="post-card" id="post-${post.id}"><div class="post-author">${avatar({ name: post.nickname, avatar_color: post.avatar_color })}<div><strong>${esc(post.nickname)}</strong>${post.graduation_year ? `<span class="year-tag">${post.graduation_year}届</span>` : ""}<small>${dateLabel(post.created_at)}</small></div><span class="post-number">#${String(post.id).padStart(3, "0")}</span></div><p class="post-body">${esc(post.content)}</p>${post.image_url ? `<button class="post-image" data-post-image="${esc(post.image_url)}" aria-label="查看帖子配图"><img src="${esc(post.image_url)}" alt="${esc(post.nickname)}的帖子配图" loading="lazy"/></button>` : ""}<div class="post-bottom"><button class="text-link reply-toggle" data-post="${post.id}" aria-expanded="false">${icon("chat")}<span>回复${post.reply_count ? `（${post.reply_count}）` : ""}</span></button><span>一句回应，也是一次重逢</span></div><div class="replies" id="replies-${post.id}" hidden></div></article>`;
}
export async function forum(url) {
  const page = Math.max(1, Number(url.searchParams.get("page")) || 1);
  const keyword = url.searchParams.get("keyword") || "";
  const sort = url.searchParams.get("sort") === "hot" ? "hot" : "latest";
  const period = url.searchParams.get("period") === "week" ? "week" : "all";
  const [result, ranking] = await Promise.all([
    api(`/posts?page=${page}&page_size=10&keyword=${encodeURIComponent(keyword)}&sort=${sort}`),
    api(`/posts/ranking?period=${period}`),
  ]);
  const forumPath = (changes = {}) => {
    const params = new URLSearchParams({keyword,sort,period,...changes});
    return `/forum?${params}`;
  };
  return {
    html: `<div class="wrap page-content">${sectionHead("THE ALUMNI COMMON ROOM", "同窗之间，总有话想说", "想说的、想找的、想聊的，都发在这里。把这里当作课间走廊，和老同学聊聊近况。")}<form id="post-search" class="post-search"><label for="post-keyword" class="sr-only">搜索帖子内容或作者</label><input id="post-keyword" name="keyword" maxlength="100" value="${esc(keyword)}" placeholder="搜索帖子内容或作者…"/><button class="button primary" type="submit">${icon("search")}搜索帖子</button>${keyword ? '<a href="/forum" class="text-link">清空</a>' : ""}</form><div class="forum-layout"><section><div class="composer-card"><div class="composer-title">${icon("chat")}<h2>今天，想和同窗说些什么？</h2><small>登录后以校友卡身份发帖</small></div><form id="post-form"><div class="form-grid"><p class="subtle">姓名与届别使用你的校友卡信息。<a href="/me">前往个人中心</a></p></div><label class="field"><span class="sr-only">帖子正文</span><textarea name="content" required maxlength="500" rows="4" placeholder="想对母校、老师或同学说点什么…"></textarea><small class="character-count"><span id="post-count">0</span> / 500</small></label><div id="post-image-picker" hidden>${imagePicker()}</div><div class="composer-bottom"><button class="text-link" id="toggle-post-image" type="button">${icon("image")}配一张图</button><span>每一句近况，都有人愿意听。</span><button class="button primary" type="submit">发布 ${icon("arrow")}</button></div><p class="form-error" role="alert" tabindex="-1" hidden></p></form></div><div class="section-heading post-list-heading"><h2>${keyword ? "搜索结果" : "同窗动态"} <span class="count-label">${result.total} 条</span></h2><div class="category-filters"><button class="chip ${sort === "latest" ? "active" : ""}" data-post-sort="latest">最新</button><button class="chip ${sort === "hot" ? "active" : ""}" data-post-sort="hot">回复最多</button></div></div><div class="post-list">${result.items.map(postCard).join("") || empty("课间走廊还很安静", "来留个言，开启今天的第一段同窗对话。")}</div>${pagination(page, result.total, 10)}</section><aside class="forum-aside"><section class="forum-ranking"><span class="eyebrow">ALUMNI LEADERBOARD</span><h2>同窗热议榜</h2><div class="category-filters"><button class="chip ${period === "all" ? "active" : ""}" data-ranking-period="all">总榜</button><button class="chip ${period === "week" ? "active" : ""}" data-ranking-period="week">近 7 天</button></div><p class="subtle">${period === "week" ? "近 7 天发布的帖子，" : "全部帖子，"}按有效回复数排序</p><ol>${ranking.items.map(p => `<li><a href="/forum?keyword=${encodeURIComponent(p.content.slice(0, 60))}"><strong>${esc(p.content.slice(0, 40))}</strong><span>${esc(p.nickname)} · ${p.reply_count} 条回复</span></a></li>`).join("") || '<li class="subtle">暂时还没有上榜帖子</li>'}</ol></section><div class="forum-welcome"><span class="eyebrow">A PLACE FOR ALL OF US</span><h3>话题不设限，<br>同窗情常在。</h3><p>聊母校，也聊远方。<br>分享喜悦，也诉说想念。</p><div class="forum-prompt">${icon("book")}那些年，最难忘的一节课</div><div class="forum-prompt">${icon("pin")}在同一座城市的你们</div><div class="forum-prompt">${icon("school")}想给学弟学妹的一点建议</div></div><div class="community-guide"><span class="eyebrow">KIND WORDS, WARM HEARTS</span><h3>让这里，像母校一样温暖</h3><p>尊重每一位同窗，友善表达。请勿发布他人隐私、广告或不当内容，公开内容采用先发后审。</p><a href="/privacy" class="text-link">社区与隐私说明 ${icon("up")}</a></div><div class="forum-mini-cta">还没有自己的校友卡？<a href="/register">写下近况，领取一份归属 ${icon("arrow")}</a></div></aside></div></div>`,
    bind(root) {
      const search = root.querySelector("#post-search");
      search.onsubmit = (event) => { event.preventDefault(); navigate(forumPath({keyword:search.elements.keyword.value.trim(),page:1})); };
      root.querySelectorAll("[data-post-sort]").forEach(button => button.onclick = () => navigate(forumPath({sort:button.dataset.postSort,page:1})));
      root.querySelectorAll("[data-ranking-period]").forEach(button => button.onclick = () => navigate(forumPath({period:button.dataset.rankingPeriod})));
      const form = root.querySelector("#post-form");
      bindPicker(form);
      const key = requestKey();
      form.elements.content.oninput = () =>
        (root.querySelector("#post-count").textContent =
          form.elements.content.value.length);
      root.querySelector("#toggle-post-image").onclick = () => {
        const box = root.querySelector("#post-image-picker");
        box.hidden = !box.hidden;
        if (box.hidden) {
          form.elements.image.value = "";
          form.elements.image.dispatchEvent(new Event("change"));
        }
      };
      form.onsubmit = (e) => {
        e.preventDefault();
        if (!session()) { navigate("/login?next=/forum"); return; }
        busy(form, async () => {
          const data = new FormData(form);
          if (form.elements.image.files[0])
            data.set(
              "image",
              await selectedImage(form),
            );
          else data.delete("image");
          await api("/posts", {
            method: "POST",
            headers: { "Idempotency-Key": key },
            body: data,
          });
          navigate("/forum");
          toast("新帖子已发布，等一位同窗的回应。");
        });
      };
      root
        .querySelectorAll("[data-page]")
        .forEach(
          (b) => (b.onclick = () => navigate(forumPath({page:b.dataset.page}))),
        );
      root
        .querySelectorAll("[data-post-image]")
        .forEach(
          (b) =>
            (b.onclick = () =>
              modal(
                "帖子配图",
                `<img class="lightbox-image" src="${esc(b.dataset.postImage)}" alt="帖子配图"/>`,
                "lightbox",
              )),
        );
      root.querySelectorAll(".reply-toggle").forEach(
        (b) =>
          (b.onclick = async () => {
            const box = root.querySelector(`#replies-${b.dataset.post}`);
            if (!box.hidden) {
              box.hidden = true;
              b.setAttribute("aria-expanded", "false");
              return;
            }
            box.hidden = false;
            b.setAttribute("aria-expanded", "true");
            box.innerHTML =
              '<div class="subtle reply-loading">正在打开回复…</div>';
            let replyPage = 1;
            const load = async () => {
              try {
                const replies = await api(
                  `/posts/${b.dataset.post}/replies?page=${replyPage}&page_size=20`,
                );
                box.innerHTML = `<div class="reply-list">${replies.items.map((r) => `<div class="reply-item">${avatar({ name: r.nickname }, "small")}<div><strong>${esc(r.nickname)}</strong><small>${dateLabel(r.created_at)}</small><p>${esc(r.content)}</p></div></div>`).join("") || '<p class="subtle">还没有回复，来说第一句话吧。</p>'}</div><div class="reply-pagination">${replyPage > 1 ? '<button class="text-link" data-reply-page="prev">← 上一页回复</button>' : ""}${replies.has_more ? '<button class="text-link" data-reply-page="next">下一页回复 →</button>' : ""}</div><form class="reply-form"><small>以当前校友卡姓名回复</small><textarea name="content" rows="2" required maxlength="500" placeholder="和这位同窗说句话…" aria-label="回复内容"></textarea><p class="form-error" role="alert" tabindex="-1" hidden></p><button type="submit" class="button primary">发送回复 ${icon("arrow")}</button></form>`;
                const rf = box.querySelector("form");
                const replyKey = requestKey();
                rf.onsubmit = (e) => {
                  e.preventDefault();
                  if (!session()) { navigate("/login?next=/forum"); return; }
                  busy(rf, async () => {
                    const reply = await api(
                      `/posts/${b.dataset.post}/replies`,
                      {
                        method: "POST",
                        headers: { "Idempotency-Key": replyKey },
                        body: JSON.stringify(
                          Object.fromEntries(new FormData(rf)),
                        ),
                      },
                    );
                    b.querySelector("span").textContent =
                      `回复（${reply.reply_count}）`;
                    replyPage = Math.max(1, Math.ceil(reply.reply_count / 20));
                    await load();
                    toast("回复已送达");
                  });
                };
                box.querySelectorAll("[data-reply-page]").forEach(
                  (button) =>
                    (button.onclick = () => {
                      replyPage += button.dataset.replyPage === "next" ? 1 : -1;
                      load();
                    }),
                );
              } catch (err) {
                box.innerHTML = `<p class="form-error">${esc(err.message)}</p><button class="button outline reply-retry">重试</button>`;
                box.querySelector("button").onclick = load;
              }
            };
            await load();
          }),
      );
    },
  };
}
