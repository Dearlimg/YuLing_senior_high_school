import qrcode from "./third-party/qrcode.js";
import { modal, toast } from "./ui.js";
import { shareURL, shareContext, track } from "./growth.js";

function lines(ctx, value, x, y, width, lineHeight, maxLines = 3) {
  let line = "", count = 0;
  for (const char of Array.from(String(value || ""))) {
    if (ctx.measureText(line + char).width > width) {
      ctx.fillText(line, x, y + count * lineHeight); line = ""; count++;
      if (count >= maxLines) return;
    }
    line += char;
  }
  if (line) ctx.fillText(line, x, y + count * lineHeight);
}

function drawQR(ctx, url, x, y) {
  const qr = qrcode(0,"M");qr.addData(url);qr.make();
  const count = qr.getModuleCount(), scale = Math.max(2,Math.floor(240/(count+8)));
  const size = (count+8)*scale;
  ctx.fillStyle="#fff";ctx.fillRect(x,y,size,size);
  ctx.fillStyle="#392c2b";
  for(let row=0;row<count;row++) for(let col=0;col<count;col++) {
    if(qr.isDark(row,col))ctx.fillRect(x+(col+4)*scale,y+(row+4)*scale,scale,scale);
  }
  return size;
}

export async function openPoster(person,kind="card",count=0) {
  try {
    const url=shareURL(person,kind==="class"?"class":"card");
    const canvas=document.createElement("canvas");canvas.width=900;canvas.height=1260;
    const ctx=canvas.getContext("2d");
    const gradient=ctx.createLinearGradient(0,0,900,1260);gradient.addColorStop(0,"#faf5e9");gradient.addColorStop(1,"#eee4d2");
    ctx.fillStyle=gradient;ctx.fillRect(0,0,900,1260);
    ctx.strokeStyle="#c8ac78";ctx.lineWidth=2;ctx.strokeRect(34,34,832,1192);
    ctx.fillStyle="#741f31";ctx.fillRect(34,34,832,200);
    const mark = new Image();mark.src="/assets/mark.svg";
    await mark.decode();ctx.fillStyle="#fff7e7";ctx.beginPath();ctx.arc(110,126,45,0,Math.PI*2);ctx.fill();ctx.drawImage(mark,70,86,80,80);
    ctx.fillStyle="#fff7e7";ctx.font='bold 32px "Microsoft YaHei",sans-serif';ctx.fillText("陕西省榆林中学",175,123);
    ctx.font='17px Georgia,serif';ctx.fillText("YULIN HIGH SCHOOL · SINCE 1903",175,158);
    ctx.fillStyle="#8c7044";ctx.font='19px Georgia,serif';ctx.fillText(kind==="class"?"OUR CLASS, TOGETHER AGAIN":"ONE OF US, FOREVER ALUMNI",78,296);
    ctx.fillStyle="#382927";ctx.font=`bold ${kind!=="class" && Array.from(person.name||"").length>22?44:60}px "Microsoft YaHei",sans-serif`;
    lines(ctx,kind==="class"?"同班同学，等你报到。":person.name,78,390,740,76,2);
    ctx.font='30px "Microsoft YaHei",sans-serif';ctx.fillStyle="#78634c";
    lines(ctx,`${person.graduation_year} 届 · ${person.class_name || "榆中校友"}${person.identity==="student"?" · 在读":""}`,78,555,740,43,2);
    if(kind==="class") {
      ctx.font='bold 110px Georgia,serif';ctx.fillStyle="#741f31";ctx.fillText(String(count),78,790);
      ctx.font='28px "Microsoft YaHei",sans-serif';ctx.fillStyle="#78634c";ctx.fillText("位同学已报到，下一位就是你。",78,850);
    } else {
      // Render a public-field allowlist directly; never capture the owner page or edit-code DOM.
      ctx.font='32px "Microsoft YaHei",sans-serif';ctx.fillStyle="#382927";
      const school=person.identity==="student"?person.dream_university:person.university;
      lines(ctx,`${person.identity==="student"&&school?"理想大学 · ":""}${school||"从榆中出发，奔赴各自的山海。"}`,78,690,740,46,2);
      ctx.font='25px "Microsoft YaHei",sans-serif';ctx.fillStyle="#78634c";
      lines(ctx,person.message||"从此山海，皆有同窗。",78,810,730,38,3);
    }
    ctx.strokeStyle="#cab68e";ctx.beginPath();ctx.moveTo(78,950);ctx.lineTo(820,950);ctx.stroke();
    drawQR(ctx,url.href,78,982);
    ctx.fillStyle="#741f31";ctx.font='bold 28px "Microsoft YaHei",sans-serif';ctx.fillText("榆中校友之家",370,1040);
    ctx.fillStyle="#78634c";ctx.font='23px "Microsoft YaHei",sans-serif';
    ctx.fillText(kind==="class"?"扫码看看同班都有谁":"扫码领取你的校友卡",370,1090);
    ctx.font='18px "Microsoft YaHei",sans-serif';ctx.fillText("怀天下 · 求真知 · 报国家",370,1142);
    modal(kind==="class"?"班级召集海报":"我的校友卡海报",'<div id="poster-preview" class="poster-preview"></div><div class="growth-actions"><button class="button primary" id="download-poster">保存海报图片</button><button class="button outline" id="share-poster-file">分享图片</button></div><p class="subtle">也可长按图片保存后发给同学。二维码带有班级或个人卡片入口，海报不包含联系方式和管理码。</p>');
    const preview = document.createElement("img");preview.src=canvas.toDataURL("image/png");preview.alt=kind==="class"?`${person.graduation_year}届${person.class_name}班级召集海报`:`${person.name}的校友卡海报`;
    document.querySelector("#poster-preview").append(preview);
    track("poster_view",person,shareContext(url));
    document.querySelector("#download-poster").onclick=()=>{
      const a=document.createElement("a");a.href=preview.src;a.download=kind==="class"?"榆中班级召集海报.png":"我的榆中校友卡.png";a.click();
      track("poster_save",person,shareContext(url));toast("已发起保存；若未下载，请长按海报保存图片。");
    };
    document.querySelector("#share-poster-file").onclick=async()=>{
      const blob=await new Promise(resolve=>canvas.toBlob(resolve,"image/png"));
      const file=new File([blob],"榆中校友之家.png",{type:"image/png"});
      if(!navigator.canShare?.({files:[file]})){toast("当前浏览器暂不支持图片分享，请保存后发送。");return;}
      track("share_click",person,shareContext(url));
      try {await navigator.share({files:[file],title:"榆中校友之家"});} catch(error) {if(error.name!=="AbortError")toast("分享未完成，请保存图片后发送。");}
    };
  } catch(error) {toast("海报暂时生成失败，请重试。");}
}
