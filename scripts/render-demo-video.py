#!/usr/bin/env python3
"""Render a VoiceRewrite walkthrough from UI screenshots and scripted examples.

Requires Pillow and FFmpeg.
"""
import argparse
from functools import lru_cache
import math
import os
from pathlib import Path
import shutil
import subprocess
from PIL import Image, ImageDraw, ImageFont, ImageFilter

W,H,FPS,DURATION=1280,720,20,30
BG='#0C1221'; PANEL='#151E30'; BORDER='#2B3850'; WHITE='#F8FAFD'; MUTED='#94A5BB'
ROSE='#E15E9B'; ROSE_LIGHT='#FFEAF4'; TEAL='#55D5BD'; DARK='#1F2937'

FONT_CJK=[ '/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc',
           '/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc',
           'C:/Windows/Fonts/msyh.ttc','C:/Windows/Fonts/msyh.ttf']
FONT_LATIN=['/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf','C:/Windows/Fonts/arial.ttf']

def font(size):
    for f in FONT_CJK+FONT_LATIN:
        if os.path.isfile(f): return ImageFont.truetype(f,size)
    raise RuntimeError('No Chinese-capable font found. Install fonts-noto-cjk before rendering.')
F={s:font(s) for s in [13,15,17,19,20,21,22,24,25,28,30,33,38,48,58]}

def rr(d,rect,r,fill,outline=None,width=1):
    d.rounded_rectangle(tuple(map(int,rect)),radius=r,fill=fill,outline=outline,width=width)

def text(d,pos,t,size=21,fill=WHITE): d.text(pos,t,font=F[size],fill=fill)

@lru_cache(maxsize=12)
def shadow_cached(rect,r,opacity,blur):
    mask=Image.new('RGBA',(W,H));dd=ImageDraw.Draw(mask)
    dd.rounded_rectangle(rect,radius=r,fill=(0,0,0,opacity));return mask.filter(ImageFilter.GaussianBlur(blur))

def shadow(canvas,rect,r=16,opacity=65,blur=15):
    canvas.alpha_composite(shadow_cached(rect,r,opacity,blur))

def fit_image(canvas,im,box):
    x,y,w,h=box; factor=min(w/im.width,h/im.height)
    iw,ih=int(im.width*factor),int(im.height*factor)
    if iw<1 or ih<1:return
    canvas.alpha_composite(im.resize((iw,ih),Image.Resampling.LANCZOS).convert('RGBA'),(int(x+(w-iw)/2),int(y+(h-ih)/2)))

def interpolate(v,t0,t1,t): return min(1,max(0,(t-t0)/(t1-t0)))

def ease(v):return v*v*(3-2*v)

def pill(d,box,label,bg,fg=WHITE,outline=None,size=17):
    rr(d,box,15,bg,outline,1);x,y,*_=box;text(d,(x+13,y+8),label,size,fg)

def mouse(canvas,d,which='none',held=False):
    # Stylized top/side-view mouse: two distinct lateral thumb buttons.
    x,y=987,265
    shadow(canvas,(x-15,y-15,x+214,y+248),38,55,17)
    rr(d,(x,y,x+185,y+220),65,'#293449','#455570',2)
    d.line((x+90,y+15,x+90,y+91),fill='#68768A',width=3)
    rr(d,(x+80,y+33,x+101,y+73),10,'#8E9CB2')
    rr(d,(x-20,y+100,x+49,y+139),14,ROSE if (which=='upper' and held) else '#53627B')
    rr(d,(x-20,y+153,x+49,y+192),14,TEAL if (which=='lower' and held) else '#53627B')
    # Buttons on the outer left edge, not the two main mouse buttons.
    text(d,(x-1,y+106),'5',19,'#111827' if which=='upper' and held else WHITE)
    text(d,(x-1,y+159),'4',19,'#111827' if which=='lower' and held else WHITE)
    if which in ('upper','lower'):
        iy=y+120 if which=='upper' else y+173
        color=ROSE if which=='upper' else TEAL
        d.ellipse((x-27,iy-11,x-3,iy+13),outline=color,width=3)
        if held:
            for off in range(3):d.arc((x-40-off*8,iy-24-off*8,x+12+off*8,iy+26+off*8),105,255,fill=color,width=2)
    text(d,(925,511),'上侧键  Mouse5',17,ROSE if which=='upper' else '#B5C4D8')
    text(d,(925,539),'下侧键  Mouse4',17,TEAL if which=='lower' else '#B5C4D8')
    if held: pill(d,(953,579,1179,622),'  ● 按住正在说话', '#343048',ROSE,ROSE)
    else: pill(d,(953,579,1179,622),'  ○ 松开完成', '#27364A',MUTED,'#42516A')

def header(canvas,d,act,t):
    rr(d,(30,24,1250,101),24,'#121D30','#283751',1)
    rr(d,(47,43,87,83),10,'#BA4A82');text(d,(61,46),'V',25,WHITE)
    text(d,(103,38),'VoiceRewrite',33,WHITE)
    text(d,(358,49),'按住说话，松开完成',21,'#C0CADE')
    labels=['01 语音输入','02 选中改写','03 选中解释']
    for i,label in enumerate(labels):
        x=740+i*168
        col=TEAL if (act==i) else '#25344A'
        rr(d,(x,44,x+155,83),17,col)
        text(d,(x+10,53),label,17,'#0D2632' if act==i else '#8C9AB0')
    rr(d,(38,107,1242,110),2,'#26374C')
    rr(d,(38,107,38+int((1204)*(t/DURATION)),110),2,TEAL)

def editor_shell(canvas,d,desc='记事本  ·  演示编辑区'):
    shadow(canvas,(37,145,890,644),22,60,16)
    rr(d,(40,143,882,638),18,'#F7F8FB')
    rr(d,(40,143,882,190),18,'#E8ECF3')
    d.rectangle((40,171,882,190),fill='#E8ECF3')
    d.ellipse((60,162,70,172),fill='#E35F89');d.ellipse((79,162,89,172),fill='#E6C25D');d.ellipse((98,162,108,172),fill='#6ECBBA')
    text(d,(129,151),desc,19,'#354258')
    d.line((64,219,856,219),fill='#E3E9EF',width=2)
    text(d,(70,195),'文件     编辑     查看',15,'#65758A')
    return (70,252)

def body_header(d,title,sub):
    text(d,(68,247),title,28,DARK)
    text(d,(68,297),sub,17,'#697C91')

def body_text(d,contents,y=352,highlight=False,appear=None):
    if not contents:return
    if highlight:
        width=min(760,max(220,int(len(contents)*25)))
        rr(d,(69,y-5,72+width,y+39),6,'#BBDEFF')
    text(d,(76,y),contents,25,'#1E293B')
    if appear is not None:
        x=min(831,76+len(contents)*25)
        if int(appear*3)%2==0:d.line((x,y+1,x,y+32),fill='#BC4B84',width=3)

def voice_bubble(d,phrase,kind,t):
    color=ROSE if kind=='upper' else TEAL
    rr(d,(78,471,820,573),16,'#18263A')
    d.ellipse((94,495,148,548),fill='#2A4257')
    for k in range(7):
        level=10+int((math.sin(t*8+k*1.35)**2)*(25+(k%3)*5))
        x=104+k*6
        d.line((x,521-level/2,x,521+level/2),fill=color,width=3)
    text(d,(168,486),'你说：',17,'#ACBCD1')
    text(d,(168,517),'“'+phrase+'”',21,WHITE)

def side_panel(canvas,d,action,which,held,note):
    rr(d,(904,143,1240,638),22,PANEL,'#304058',2)
    text(d,(930,176),action,24,WHITE)
    text(d,(930,219),note,17,MUTED)
    mouse(canvas,d,which,held)

@lru_cache(maxsize=1)
def overlay_img_cached():
    im=Image.open(str(RECORDING_PATH)).convert('RGBA')
    pix=im.load()
    for y in range(im.height):
        for x in range(im.width):
            r,g,b,a=pix[x,y]
            if b>r*1.55 and b>g*1.22 and b>75:
                pix[x,y]=(r,g,b,0)
    iw,ih=267,57
    factor=min(iw/im.width,ih/im.height)
    iw,ih=int(im.width*factor),int(im.height*factor)
    return im.resize((iw,ih),Image.Resampling.LANCZOS)

def overlay(canvas,recording_img,t):
    im=overlay_img_cached()
    canvas.alpha_composite(im,(333+(267-im.width)//2,575+(57-im.height)//2))

def chip(d,label,color):
    rr(d,(69,397,796,452),13,'#ECF9F5' if color==TEAL else '#FFF2F7')
    text(d,(88,411),label,20,'#28675E' if color==TEAL else '#A74772')

def answer_card(canvas,d):
    shadow(canvas,(126,333,814,614),16,55,11)
    rr(d,(124,329,816,611),15,'#FAFBFD','#DDE3ED',2)
    rr(d,(124,329,816,369),14,'#E9EDF3')
    d.rectangle((124,352,816,369),fill='#E9EDF3')
    text(d,(147,335),'VoiceRewrite · AI 回答',17,'#303F52')
    text(d,(146,387),'缓存是什么？',24,'#1D2B40')
    for yy,content in [(431,'缓存会暂时保存最近使用的数据或计算结果。'),(467,'下次需要时可以直接读取，减少等待和重复计算。'),(503,'这是一条解释，不会修改你原来选中的文字。')]:
        text(d,(147,yy),content,19,'#34455B')
    rr(d,(623,557,779,594),10,'#C85288')
    text(d,(659,566),'复制结果',17,WHITE)

def scene(canvas,recording_img,t):
    d=ImageDraw.Draw(canvas)
    d.rectangle((0,0,W,H),fill=BG)
    # subtle grid & corner glow
    for x in range(0,W,64):d.line((x,120,x,H),fill='#121B2C',width=1)
    for y in range(130,H,64):d.line((0,y,W,y),fill='#121B2C',width=1)
    act=0 if t<11 else 1 if t<20 else 2
    header(canvas,d,act,t)
    editor_shell(canvas,d)
    if t<11:
        body_header(d,'光标在哪，文字就写到哪','示例：在任意编辑器中直接输入中文')
        if t<2.4:
            body_text(d,'|',352)
            side_panel(canvas,d,'01 · 语音输入','lower',False,'下侧键 Mouse4')
            pill(d,(74,477,740,527),'   将光标放在输入位置 → 按住下侧键', '#E7F7F5','#1E7773')
        elif t<6.5:
            body_text(d,'',352)
            side_panel(canvas,d,'01 · 语音输入','lower',True,'下侧键 Mouse4')
            voice_bubble(d,'下周二下午三点开产品会议。','lower',t)
            overlay(canvas,recording_img,t)
        elif t<7.5:
            side_panel(canvas,d,'01 · 语音输入','lower',False,'松开，开始识别')
            body_text(d,'',352)
            chip(d,'识别中  ·  正在转换语音…',TEAL)
        else:
            side_panel(canvas,d,'01 · 语音输入','lower',False,'松开后自动输入')
            msg='下周二下午三点开产品会议。'
            n=int(interpolate(0,7.5,9.65,t)*len(msg))+1
            body_text(d,msg[:n],352,appear=t)
            chip(d,'✓  语音已转成文字，并插入光标位置',TEAL)
    elif t<20:
        body_header(d,'选中文字，然后说你想怎么改','示例：用一句话要求更正式、更简洁')
        selected='这个功能做得还行，但速度有点慢。'
        if t<16.2:
            body_text(d,selected,353,highlight=True)
            side_panel(canvas,d,'02 · 选中改写','upper',t>=12.6,'上侧键 Mouse5')
            if t>=12.6:
                voice_bubble(d,'改得更正式、更简洁。','upper',t)
                overlay(canvas,recording_img,t)
            else:
                pill(d,(74,478,765,530),'   选中原文 → 按住上侧键 → 说改写指令', '#FFF0F6','#AB3E70')
        elif t<17:
            body_text(d,selected,353,highlight=True)
            side_panel(canvas,d,'02 · 选中改写','upper',False,'松开，执行改写')
            chip(d,'正在识别指令并生成改写…',ROSE)
        else:
            body_text(d,'该功能基本满足需求，但仍有性能优化空间。',353)
            side_panel(canvas,d,'02 · 选中改写','upper',False,'确认目标后应用结果')
            chip(d,'✓  改写结果已替换原选中文字',TEAL)
    else:
        body_header(d,'也能提问，而不改动原文','同一颗上侧键，语音指令决定是改写还是解释')
        selected='模型会缓存最近的计算结果。'
        body_text(d,selected,353,highlight=True)
        if t<23.8:
            side_panel(canvas,d,'03 · 选中解释','upper',t>=21,'上侧键 Mouse5')
            if t>=21:
                voice_bubble(d,'解释一下这里的缓存。','upper',t)
                overlay(canvas,recording_img,t)
            else:
                pill(d,(74,477,766,527),'   选中不懂的句子 → 按住上侧键提问', '#FFF0F6','#AB3E70')
        elif t<24.4:
            side_panel(canvas,d,'03 · 选中解释','upper',False,'松开，生成解释')
            chip(d,'正在回答问题，不修改原文…',ROSE)
        else:
            side_panel(canvas,d,'03 · 选中解释','upper',False,'答案显示在结果卡片')
            answer_card(canvas,d)
    # bottom caption on screen to avoid making synthetic workflow appear actual.
    text(d,(45,665),'交互流程示意 · 编辑器与语音指令为演示文本 · 录音浮条取自真实 Windows EXE 截图',15,'#7E90AA')
    text(d,(1100,665),'30s DEMO',15,'#9DB0C6')
    return canvas

def build(assets,output,fps=FPS,duration=DURATION):
    recpath=assets/'recording-overlay.png'
    if not recpath.is_file():raise SystemExit('Missing '+str(recpath))
    global RECORDING_PATH
    RECORDING_PATH=recpath
    recording=Image.open(recpath).convert('RGBA')
    output.mkdir(parents=True,exist_ok=True)
    mp4=output/'voicerewrite-demo.mp4'
    args=['ffmpeg','-y','-hide_banner','-loglevel','error','-f','rawvideo','-vcodec','rawvideo','-s',f'{W}x{H}', '-pix_fmt','rgb24','-r',str(fps),'-i','-','-an','-c:v','libx264','-pix_fmt','yuv420p','-preset','medium','-crf','18','-movflags','+faststart',str(mp4)]
    p=subprocess.Popen(args,stdin=subprocess.PIPE,stderr=subprocess.PIPE)
    try:
        for i in range(fps*duration):
            frame=scene(Image.new('RGBA',(W,H),BG),recording,i/fps).convert('RGB')
            p.stdin.write(frame.tobytes())
        p.stdin.close()
        err=p.stderr.read().decode('utf8','replace')
        ret=p.wait()
    except Exception:
        if p.stdin and not p.stdin.closed:p.stdin.close()
        p.kill();p.wait();raise
    if ret:raise RuntimeError('ffmpeg encoding failed: '+err[-900:])
    webp=output/'voicerewrite-preview.webp'
    subprocess.run(['ffmpeg','-y','-hide_banner','-loglevel','error','-i',str(mp4),
        '-vf','fps=12,scale=1280:-2:flags=lanczos', '-c:v','libwebp_anim',
        '-lossless','0','-quality','90','-compression_level','4',
        '-loop','0','-an',str(webp)], check=True)
    return mp4,webp

if __name__=='__main__':
    parser=argparse.ArgumentParser()
    parser.add_argument('--assets',default='docs/ui',type=Path)
    parser.add_argument('--output',default='docs/demo',type=Path)
    parser.add_argument('--fps',default=FPS,type=int)
    parser.add_argument('--duration',default=DURATION,type=int)
    v=parser.parse_args()
    build(v.assets,v.output,v.fps,v.duration)