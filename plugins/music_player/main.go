package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type request struct {
	Hook   string         `json:"hook"`
	Site   site           `json:"site"`
	Build  build          `json:"build"`
	Config map[string]any `json:"plugin_config"`
}

type site struct {
	BasePath string `json:"base_path"`
}

type build struct {
	BasePath string `json:"base_path"`
}

type response struct {
	HeadHTML string `json:"head_html,omitempty"`
	Message  string `json:"message,omitempty"`
	Error    string `json:"error,omitempty"`
}

func main() {
	var req request
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		writeResp(response{Error: fmt.Sprintf("decode request failed: %v", err)})
		os.Exit(1)
	}
	if req.Hook != "before_page_render" {
		writeResp(response{Message: "hook skipped"})
		return
	}

	basePath := strings.TrimSpace(req.Build.BasePath)
	if basePath == "" {
		basePath = strings.TrimSpace(req.Site.BasePath)
	}

	src := configString(req.Config, "src", withBase(basePath, "/static/plugins/music_player/music.mp3"))
	src = normalizeMediaSrc(basePath, src)
	title := configString(req.Config, "title", "Now Playing")
	volume := configFloat(req.Config, "volume", 0.8)
	if volume < 0 {
		volume = 0
	}
	if volume > 1 {
		volume = 1
	}

	cfg := map[string]any{
		"src":    src,
		"title":  title,
		"volume": volume,
	}
	b, _ := json.Marshal(cfg)
	head := `<style id="folio-music-player-style">
#folio-music-player{position:fixed;right:14px;top:25vh;bottom:auto;z-index:9999;width:220px;background:var(--paper,var(--bg,#ffffff));color:var(--ink,#171717);border:2px solid var(--line,rgba(0,0,0,.25));border-radius:12px;box-shadow:0 8px 24px rgba(0,0,0,.22);padding:8px 10px;font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;touch-action:none;user-select:none}
#folio-music-player.dragging{opacity:.92;cursor:grabbing}
#folio-music-player .head{display:flex;align-items:center;justify-content:space-between;margin:0 0 6px;cursor:grab}
#folio-music-player .title{font-size:11px;opacity:.86;margin:0;overflow:hidden;white-space:nowrap;text-overflow:ellipsis;padding-right:6px}
#folio-music-player .actions{display:flex;gap:4px}
#folio-music-player button{appearance:none;border:1px solid var(--line,rgba(0,0,0,.25));background:rgba(0,0,0,.06);color:var(--ink,#171717);width:22px;height:22px;border-radius:7px;font-size:13px;line-height:1;cursor:pointer}
#folio-music-player button:hover{background:rgba(0,0,0,.12)}
#folio-music-player audio{width:100%;height:34px}
#folio-music-player-mini{position:fixed;right:14px;top:25vh;bottom:auto;z-index:9998;width:40px;height:40px;border-radius:999px;border:2px solid var(--line,rgba(0,0,0,.25));background:var(--paper,var(--bg,#fff));color:var(--ink,#171717);box-shadow:0 8px 20px rgba(0,0,0,.22);display:none;cursor:pointer}
@media (max-width:640px){#folio-music-player{width:200px}}
</style>
<script id="folio-music-player-script">
(function(){
  var cfg=` + string(b) + `;
  if(!cfg.src){return;}

  if(!window.__folioMusicTurboHook){
    window.__folioMusicTurboHook=true;
    document.addEventListener('turbo:before-render',function(e){
      if(!e||!e.detail||!e.detail.newBody){return;}
      var nb=e.detail.newBody;
      if(!nb.querySelector('#folio-music-player')){
        var ph=document.createElement('div');
        ph.id='folio-music-player';
        ph.setAttribute('data-turbo-permanent','');
        nb.appendChild(ph);
      }
      if(!nb.querySelector('#folio-music-player-mini')){
        var mh=document.createElement('button');
        mh.id='folio-music-player-mini';
        mh.setAttribute('data-turbo-permanent','');
        nb.appendChild(mh);
      }
    });
  }
  if(document.getElementById('folio-music-player')){return;}

  var POS_KEY='folio.music.pos.v1';
  var MINI_KEY='folio.music.mini.v1';
  var STATE_KEY='folio.music.state.v1';
  var STATE_KEY_SESSION='folio.music.state.session.v1';
  var STATE_NAME_PREFIX='__folio_music_state__:';

  function getNameStateRaw(){
    try{
      var n=String(window.name||'');
      if(n.indexOf(STATE_NAME_PREFIX)!==0){return '';}
      return n.slice(STATE_NAME_PREFIX.length);
    }catch(_e){return '';}
  }
  function setNameStateRaw(raw){
    try{ window.name=STATE_NAME_PREFIX+raw; }catch(_e){}
  }
  function parseState(raw){
    if(!raw){return null;}
    try{
      var s=JSON.parse(raw);
      if(!s){return null;}
      if(typeof s.src==='string' && s.src!==cfg.src){return null;}
      if(typeof s.t!=='number' || !Number.isFinite(s.t)){s.t=0;}
      if(typeof s.ts!=='number' || !Number.isFinite(s.ts)){s.ts=0;}
      return s;
    }catch(_e){return null;}
  }
  function latestState(){
    var a=parseState(localStorage.getItem(STATE_KEY));
    var b=null;
    try{ b=parseState(sessionStorage.getItem(STATE_KEY_SESSION)); }catch(_e){}
    var c=parseState(getNameStateRaw());
    var arr=[a,b,c].filter(function(x){ return !!x; });
    if(arr.length===0){return null;}
    arr.sort(function(x,y){ return (y.ts||0)-(x.ts||0); });
    return arr[0];
  }

  var wrap=document.createElement('div');
  wrap.id='folio-music-player';
  wrap.setAttribute('data-turbo-permanent','');
  var head=document.createElement('div');
  head.className='head';
  var title=document.createElement('p');
  title.className='title';
  var playingLabel=cfg.title||'Now Playing';
  var pausedLabel='Paused';
  function inferTrackName(src){
    try{
      var clean=(src||'').split('?')[0].split('#')[0];
      var file=clean.substring(clean.lastIndexOf('/')+1);
      if(!file){return '';}
      file=decodeURIComponent(file);
      var dot=file.lastIndexOf('.');
      if(dot>0){file=file.substring(0,dot);}
      return file.replace(/[_-]+/g,' ').trim();
    }catch(_e){ return ''; }
  }
  var trackName=inferTrackName(cfg.src);
  function renderTitle(paused){
    var state=paused?pausedLabel:playingLabel;
    title.textContent=trackName?(state+' · '+trackName):state;
  }
  renderTitle(true);

  var actions=document.createElement('div');
  actions.className='actions';
  var btnMini=document.createElement('button');
  btnMini.type='button';
  btnMini.title='Minimize';
  btnMini.textContent='−';
  actions.appendChild(btnMini);
  head.appendChild(title);
  head.appendChild(actions);

  var audio=document.createElement('audio');
  audio.controls=true;
  audio.preload='metadata';
  audio.src=cfg.src;
  audio.loop=true;
  audio.autoplay=false;
  audio.volume=typeof cfg.volume==='number'?cfg.volume:0.8;
  audio.addEventListener('play',function(){ renderTitle(false); });
  audio.addEventListener('pause',function(){ renderTitle(true); });
  audio.addEventListener('ended',function(){ renderTitle(true); });

  var resumeState=null;
  var unlockArmed=false;
  function armUserUnlock(){
    if(unlockArmed){return;}
    unlockArmed=true;
    var evs=['pointerdown','mousedown','touchstart','keydown'];
    var handler=function(){
      audio.play().catch(function(){ return null; }).finally(function(){
        evs.forEach(function(ev){ document.removeEventListener(ev,handler,true); });
        unlockArmed=false;
      });
    };
    evs.forEach(function(ev){ document.addEventListener(ev,handler,true); });
  }
  function loadState(){
    return latestState();
  }
  function saveState(){
    var st={
      src: cfg.src,
      t: Number.isFinite(audio.currentTime)?audio.currentTime:0,
      paused: !!audio.paused,
      volume: Number.isFinite(audio.volume)?audio.volume:cfg.volume,
      ts: Date.now(),
    };
    // Do not overwrite valid progress with initial paused 0s.
    try{
      var oldRaw=localStorage.getItem(STATE_KEY);
      if(oldRaw){
        var old=JSON.parse(oldRaw);
        if(old && old.src===cfg.src && typeof old.t==='number' && old.t>1 && st.paused && st.t<0.5){
          return;
        }
      }
    }catch(_e){}
    var raw=JSON.stringify(st);
    localStorage.setItem(STATE_KEY,raw);
    try{ sessionStorage.setItem(STATE_KEY_SESSION,raw); }catch(_e){}
    setNameStateRaw(raw);
  }
  function applyResumeState(){
    resumeState=loadState();
    if(!resumeState){return;}
    if(typeof resumeState.volume==='number' && resumeState.volume>=0 && resumeState.volume<=1){
      audio.volume=resumeState.volume;
    }
    var seekTo=function(){
      if(typeof resumeState.t==='number' && resumeState.t>0 && Number.isFinite(audio.duration)){
        var max=Math.max(0,audio.duration-0.25);
        audio.currentTime=Math.min(resumeState.t,max);
      }else if(typeof resumeState.t==='number' && resumeState.t>0){
        audio.currentTime=resumeState.t;
      }
    };
    audio.addEventListener('loadedmetadata',seekTo,{once:true});
    if(audio.readyState>=1){seekTo();}
  }
  function startPlaybackFlow(){
    applyResumeState();
    if(resumeState && !resumeState.paused){
      audio.play().catch(function(){
        armUserUnlock();
        return null;
      });
    }
  }

  var miniBtn=document.createElement('button');
  miniBtn.id='folio-music-player-mini';
  miniBtn.setAttribute('data-turbo-permanent','');
  miniBtn.type='button';
  miniBtn.title='Open player';
  miniBtn.textContent='🎵';

  wrap.appendChild(head);
  wrap.appendChild(audio);
  function applyPosTo(el,x,y){
    el.style.left=x+'px';
    el.style.top=y+'px';
    el.style.right='auto';
    el.style.bottom='auto';
  }
  function applyPos(x,y){
    applyPosTo(wrap,x,y);
    applyPosTo(miniBtn,x,y);
  }
  function clamp(v,min,max){return Math.max(min,Math.min(max,v));}
  function savePosFrom(el){
    var r=el.getBoundingClientRect();
    localStorage.setItem(POS_KEY,JSON.stringify({x:r.left,y:r.top}));
  }
  function setMini(on){
    localStorage.setItem(MINI_KEY,on?'1':'0');
    wrap.style.display=on?'none':'block';
    miniBtn.style.display=on?'block':'none';
  }
  var dragging=false,dragMoved=false,startX=0,startY=0,origX=0,origY=0,target=wrap,suppressClickUntil=0;
  function shouldIgnoreClick(){ return Date.now()<suppressClickUntil; }
  btnMini.addEventListener('click',function(e){
    if(shouldIgnoreClick()){return;}
    e.stopPropagation();
    var r=wrap.getBoundingClientRect();
    applyPos(r.left,r.top);
    setMini(true);
  });
  miniBtn.addEventListener('click',function(){
    if(shouldIgnoreClick()){return;}
    var r=miniBtn.getBoundingClientRect();
    applyPos(r.left,r.top);
    setMini(false);
  });
  function beginDrag(e,t,allowButton){
    if(!allowButton && e.target && e.target.closest){
      if(e.target.closest('button') || e.target.closest('audio')){return;}
    }
    dragging=true;target=t;
    dragMoved=false;
    var point=e.touches?e.touches[0]:e;
    startX=point.clientX;startY=point.clientY;
    var r=t.getBoundingClientRect();
    origX=r.left;origY=r.top;
    t.classList.add('dragging');
  }
  function moveDrag(e){
    if(!dragging){return;}
    var point=e.touches?e.touches[0]:e;
    if(Math.abs(point.clientX-startX)>4||Math.abs(point.clientY-startY)>4){dragMoved=true;}
    var nx=origX+(point.clientX-startX);
    var ny=origY+(point.clientY-startY);
    var maxX=Math.max(0,window.innerWidth-target.offsetWidth);
    var maxY=Math.max(0,window.innerHeight-target.offsetHeight);
    nx=clamp(nx,0,maxX);ny=clamp(ny,0,maxY);
    target.style.left=nx+'px';
    target.style.top=ny+'px';
    target.style.right='auto';target.style.bottom='auto';
    e.preventDefault();
  }
  function endDrag(){
    if(!dragging){return;}
    dragging=false;
    wrap.classList.remove('dragging');
    miniBtn.classList.remove('dragging');
    if(dragMoved){suppressClickUntil=Date.now()+250;}
    savePosFrom(target);
  }

  wrap.addEventListener('mousedown',function(e){beginDrag(e,wrap,false);});
  wrap.addEventListener('touchstart',function(e){beginDrag(e,wrap,false);},{passive:false});
  miniBtn.addEventListener('mousedown',function(e){beginDrag(e,miniBtn,true);});
  miniBtn.addEventListener('touchstart',function(e){beginDrag(e,miniBtn,true);},{passive:false});
  window.addEventListener('mousemove',moveDrag,{passive:false});
  window.addEventListener('touchmove',moveDrag,{passive:false});
  window.addEventListener('mouseup',endDrag);
  window.addEventListener('touchend',endDrag);

  audio.addEventListener('timeupdate',saveState);
  audio.addEventListener('pause',saveState);
  audio.addEventListener('play',saveState);
  audio.addEventListener('volumechange',saveState);
  document.addEventListener('turbo:visit',saveState);
  document.addEventListener('turbo:before-render',saveState);
  window.addEventListener('beforeunload',saveState);
  window.addEventListener('pagehide',saveState);

  document.addEventListener('DOMContentLoaded',function(){
    document.body.appendChild(wrap);
    document.body.appendChild(miniBtn);
    var mini=localStorage.getItem(MINI_KEY)==='1';
    if(mini){setMini(true);}
    var raw=localStorage.getItem(POS_KEY);
    if(raw){
      try{
        var p=JSON.parse(raw);
        if(typeof p.x==='number'&&typeof p.y==='number'&&!(p.x<=1&&p.y<=1)){applyPos(p.x,p.y);}
      }catch(_e){}
    }
    startPlaybackFlow();
  });
})();
</script>`
	writeResp(response{HeadHTML: head, Message: "music player injected"})
}

func withBase(basePath, p string) string {
	basePath = strings.Trim(basePath, "/")
	if basePath == "" {
		return p
	}
	if strings.HasPrefix(p, "/") {
		return "/" + basePath + p
	}
	return "/" + basePath + "/" + p
}

func normalizeMediaSrc(basePath, src string) string {
	src = strings.TrimSpace(src)
	if src == "" {
		return src
	}

	// Keep absolute URLs and special schemes unchanged.
	lower := strings.ToLower(src)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(src, "//") || strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "blob:") {
		return src
	}

	cleanBase := strings.Trim(basePath, "/")
	if cleanBase == "" {
		return src
	}
	if !strings.HasPrefix(src, "/") {
		return src
	}

	prefix := "/" + cleanBase
	if src == prefix || strings.HasPrefix(src, prefix+"/") {
		return src
	}
	return prefix + src
}

func configString(cfg map[string]any, key, fallback string) string {
	if cfg == nil {
		return fallback
	}
	v, ok := cfg[key]
	if !ok {
		return fallback
	}
	s, ok := v.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func configFloat(cfg map[string]any, key string, fallback float64) float64 {
	if cfg == nil {
		return fallback
	}
	v, ok := cfg[key]
	if !ok {
		return fallback
	}
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func writeResp(resp response) {
	_ = json.NewEncoder(os.Stdout).Encode(resp)
}
