const sessionKey = "yulin-session";
export function session() {
  try {
    const value = JSON.parse(localStorage.getItem(sessionKey) || "null");
    if (value?.token && Date.parse(value.expires_at) > Date.now()) return value;
  } catch {}
  return null;
}
export function saveSession(value) {
  if (value) localStorage.setItem(sessionKey, JSON.stringify({token: value.token, expires_at: value.expires_at}));
  else localStorage.removeItem(sessionKey);
  window.dispatchEvent(new Event("auth-change"));
}
export class APIError extends Error {
  constructor(message, status) {
    super(message);
    this.status = status;
  }
}
// getRandomValues also works on ordinary HTTP origins used by the existing deployment.
export function requestKey() {
  const bytes = new Uint8Array(24);
  crypto.getRandomValues(bytes);
  return Array.from(bytes, (value) => value.toString(16).padStart(2, "0")).join(
    "",
  );
}
export async function api(path, options = {}) {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), 20000);
  try {
    const headers = new Headers(options.headers || {});
    const token = session()?.token;
    if (token) headers.set("Authorization", `Bearer ${token}`);
    if (options.body && !(options.body instanceof FormData))
      headers.set("Content-Type", "application/json");
    const response = await fetch(`/api/v1${path}`, {
      ...options,
      headers,
      signal: options.signal || controller.signal,
    });
    const body = await response.json().catch(() => ({}));
    if (response.status === 401 && session()?.token === token) saveSession(null);
    if (!response.ok || body.code !== 0)
      throw new APIError(
        body.msg || "服务暂时不可用，请稍后重试",
        response.status,
      );
    return body.data;
  } catch (error) {
    if (error.name === "AbortError")
      throw new APIError("请求超时，请检查网络后重试", 408);
    if (error instanceof APIError) throw error;
    throw new APIError("连接失败，请检查网络后重试", 0);
  } finally {
    clearTimeout(timeout);
  }
}
export async function compressImage(file) {
  if (!file || !["image/jpeg", "image/png"].includes(file.type))
    throw new Error("请选择 JPG 或 PNG 图片");
  if (file.size > 10 * 1024 * 1024) throw new Error("原图不能超过 10MB");
  const bitmap = await createImageBitmap(file).catch(() => {
    throw new Error("无法读取这张图片，请重新选择");
  });
  try {
    if (bitmap.width * bitmap.height > 40000000)
      throw new Error("图片像素过高，请选择较小的照片");
    const ratio = Math.min(1, 2048 / Math.max(bitmap.width, bitmap.height));
    const canvas = document.createElement("canvas");
    canvas.width = Math.round(bitmap.width * ratio);
    canvas.height = Math.round(bitmap.height * ratio);
    const ctx = canvas.getContext("2d");
    ctx.fillStyle = "#ffffff";
    ctx.fillRect(0, 0, canvas.width, canvas.height);
    ctx.drawImage(bitmap, 0, 0, canvas.width, canvas.height);
    const blob = await new Promise((resolve) =>
      canvas.toBlob(resolve, "image/jpeg", 0.84),
    );
    if (!blob || blob.size > 3 * 1024 * 1024)
      throw new Error("压缩失败或图片仍过大，请重新选择");
    return new File([blob], "campus.jpg", { type: "image/jpeg" });
  } finally {
    bitmap.close();
  }
}
