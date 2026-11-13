import { api, requestKey } from "./api.js";

const slug = /^[a-zA-Z0-9_-]{1,48}$/;
const claims = new Map();
const volatileVisitor = requestKey();
let attribution = null;
const sent = new Set();

function visitorID() {
  try {
    let value = JSON.parse(localStorage.getItem("yulin-visitor") || "null");
    if (!slug.test(value?.id || "") || !Number.isFinite(value.expires) || value.expires < Date.now()) {
      value = {id: requestKey(), expires: Date.now() + 90 * 86400000};
      localStorage.setItem("yulin-visitor", JSON.stringify(value));
    }
    return value.id;
  } catch { return volatileVisitor; }
}

export function captureAttribution(url) {
  const share = url.searchParams.get("share_id");
  const source = url.searchParams.get("source");
  const campaign = url.searchParams.get("campaign");
  if (slug.test(share || "") || (slug.test(source || "") && slug.test(campaign || ""))) {
    attribution = {
      share_id: slug.test(share || "") ? share : "",
      source: slug.test(source || "") ? source : "share",
      campaign: slug.test(campaign || "") ? campaign : "",
      expires: Date.now()+86400000,
    };
    try { sessionStorage.setItem("yulin-attribution", JSON.stringify(attribution)); } catch {}
  }
  if (!attribution) {
    try {
      const saved = JSON.parse(sessionStorage.getItem("yulin-attribution") || "null");
      if (saved?.expires > Date.now() && (slug.test(saved.share_id || "") ||
          (slug.test(saved.source || "") && slug.test(saved.campaign || "")))) {
        attribution = {...saved,
          share_id: slug.test(saved.share_id || "") ? saved.share_id : "",
          source: slug.test(saved.source || "") ? saved.source : "share",
          campaign: slug.test(saved.campaign || "") ? saved.campaign : "",
        };
      }
    } catch {}
  }
}

export function contextFor(person = {}) {
  if (attribution?.expires < Date.now()) attribution = null;
  return {
    visitor_id: visitorID(), source: attribution?.source || "direct",
    campaign: attribution?.campaign || "", share_id: attribution?.share_id || "",
    graduation_year: Number(person.graduation_year) || 0, class_name: person.class_name || "",
  };
}

export function track(event, person = {}, extra = {}) {
  const data = {...contextFor(person), ...extra, event};
  const key = JSON.stringify(data);
  if (sent.has(key)) return;
  sent.add(key);
  // Analytics never blocks browsing, submitting a card, or sharing.
  api("/growth/events", {method:"POST", body:JSON.stringify(data), keepalive:true})
    .catch(() => sent.delete(key));
}

export function shareURL(person, kind = "class", campaign = "") {
  return contentShareURL(kind === "class" ? "/class" : `/alumni/${person.id}`,
    kind === "class" ? {graduation_year:person.graduation_year,class_name:person.class_name} : {},
    kind === "class" ? "class_invite" : "alumni_card",campaign);
}

export function contentShareURL(path, params = {}, source = "campus_memory", campaign = "") {
  const url = new URL(path,location.origin);
  for (const [key,value] of Object.entries(params)) if (value!=="" && value!==undefined) url.searchParams.set(key,value);
  url.searchParams.set("source",source);
  const activity = campaign || (attribution?.expires > Date.now() ? attribution.campaign : "") || "reunion";
  url.searchParams.set("campaign", slug.test(activity) ? activity : "reunion");
  url.searchParams.set("share_id", requestKey());
  return url;
}

export function shareContext(url) {
  return Object.fromEntries(["source", "campaign", "share_id"].map(key => [key,url.searchParams.get(key) || ""]));
}

export function claimState(id, value) {
  if (value) claims.set(String(id), value);
  return claims.get(String(id));
}
