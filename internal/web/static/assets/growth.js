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
  if (slug.test(share || "")) {
    attribution = {
      share_id: share,
      source: slug.test(url.searchParams.get("source") || "") ? url.searchParams.get("source") : "share",
      campaign: slug.test(url.searchParams.get("campaign") || "") ? url.searchParams.get("campaign") : "",
      expires: Date.now()+86400000,
    };
    try { sessionStorage.setItem("yulin-attribution", JSON.stringify(attribution)); } catch {}
  }
  if (!attribution) {
    try {
      const saved = JSON.parse(sessionStorage.getItem("yulin-attribution") || "null");
      if (saved?.expires > Date.now() && slug.test(saved.share_id || "")) {
        attribution = {...saved,
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
  const url = new URL(kind === "class" ? "/class" : `/alumni/${person.id}`, location.origin);
  if (kind === "class") {
    url.searchParams.set("graduation_year", person.graduation_year);
    url.searchParams.set("class_name", person.class_name);
  }
  url.searchParams.set("source", kind === "class" ? "class_invite" : "alumni_card");
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
