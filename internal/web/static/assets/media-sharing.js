import { api } from "./api.js";
import { contentShareURL } from "./growth.js";
import { drawQR, lines, presentPoster } from "./poster.js";
import { toast } from "./ui.js";

export function shareCanvas(kicker) {
  const canvas=document.createElement("canvas");canvas.width=900;canvas.height=1260;
  const ctx=canvas.getContext("2d");ctx.fillStyle="#faf5e9";ctx.fillRect(0,0,900,1260);
  ctx.strokeStyle="#c8ac78";ctx.lineWidth=2;ctx.strokeRect(34,34,832,1192);
  ctx.fillStyle="#741f31";ctx.fillRect(34,34,832,150);
  ctx.fillStyle="#fff7e7";ctx.font='bold 34px "Microsoft YaHei",sans-serif';ctx.fillText("陕西省榆林中学 · 校友之家",75,105);
  ctx.font='18px Georgia,serif';ctx.fillText(kicker,75,145);
  return {canvas,ctx};
}

export function shareFooter(ctx,url,description) {
  ctx.strokeStyle="#c8ac78";ctx.beginPath();ctx.moveTo(75,955);ctx.lineTo(825,955);ctx.stroke();
  drawQR(ctx,url.href,75,975);
  ctx.fillStyle="#741f31";ctx.font='bold 28px "Microsoft YaHei",sans-serif';ctx.fillText("山河万里，仍是榆中人。",370,1040);
  ctx.fillStyle="#78634c";ctx.font='23px "Microsoft YaHei",sans-serif';lines(ctx,description,370,1100,440,34,2);
  ctx.font='18px "Microsoft YaHei",sans-serif';ctx.fillText("怀天下 · 求真知 · 报国家",370,1190);
}

export async function photoPoster(id) {
  try {
    const photo=await api(`/photos/${id}`),{canvas,ctx}=shareCanvas("MOMENTS WE SHARE · 校园记忆");
    const image=new Image();image.src=photo.image_url;await image.decode();
    const ratio=Math.min(750/image.width,500/image.height),w=image.width*ratio,h=image.height*ratio;
    ctx.drawImage(image,75+(750-w)/2,220+(500-h)/2,w,h);
    ctx.fillStyle="#382927";ctx.font='bold 32px "Microsoft YaHei",sans-serif';lines(ctx,photo.caption,75,790,750,46,2);
    ctx.fillStyle="#78634c";ctx.font='22px "Microsoft YaHei",sans-serif';lines(ctx,`${photo.uploader} · ${photo.category}`,75,914,750,30,1);
    const url=contentShareURL("/moments",{photo:id},"campus_memory");
    shareFooter(ctx,url,"扫码翻看这段校园记忆，也分享你珍藏的照片。");
    presentPoster(canvas,{url,title:"校园记忆卡片"});
  } catch {toast("校园记忆卡片暂时生成失败，请重试。");}
}

export async function universityPoster(year) {
  try {
    const result=await api(year?`/cohorts/${year}/universities`:"/universities");
    if(!result.items.length){toast("还没有实际就读院校数据，暂不生成分享图。");return;}
    const {canvas,ctx}=shareCanvas("WHERE WE GO · 大学去向");
    ctx.fillStyle="#382927";ctx.font='bold 48px "Microsoft YaHei",sans-serif';ctx.fillText(year?`${year} 届，我们去了哪里？`:"从榆中出发，我们去了哪里？",75,280);
    ctx.fillStyle="#78634c";ctx.font='25px "Microsoft YaHei",sans-serif';ctx.fillText(`${result.total} 位校友 · ${result.items.length} 所实际就读院校`,75,340);
    result.items.slice(0,8).forEach((row,i)=>{
      const y=410+i*58;ctx.fillStyle="#741f31";ctx.font='26px Georgia,serif';ctx.fillText(String(i+1).padStart(2,"0"),75,y);
      ctx.fillStyle="#382927";ctx.font='26px "Microsoft YaHei",sans-serif';lines(ctx,row.name,135,y,550,32,1);
      ctx.textAlign="right";ctx.fillText(`${row.count} 人`,820,y);ctx.textAlign="left";
    });
    ctx.fillStyle="#78634c";ctx.font='19px "Microsoft YaHei",sans-serif';ctx.fillText("展示前 8 所 · 仅统计有效校友实际院校，不含理想大学",75,915);
    const url=contentShareURL("/universities",year?{graduation_year:year}:{},"cohort_universities");
    shareFooter(ctx,url,"扫码看看同届去向，找到同校的榆中人。");
    presentPoster(canvas,{url,person:{graduation_year:year||0},title:year?`${year}届大学去向分享图`:"大学去向分享图"});
  } catch {toast("大学去向分享图暂时生成失败，请重试。");}
}
