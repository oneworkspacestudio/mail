// Synthetic logic checks against the exact JavaScript served by installed Mail.
// No browser, real account, mailbox API or user message is accessed.
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
const root = __dirname;
const source = fs.readFileSync(path.join(root, '../assets/app.js'), 'utf8');
const lines = source.split('\n');
const readable = lines.find(x=>x.startsWith('function readableBody('));
const {DOMParser: LinkedomParser} = require('linkedom');
class DOMParser extends LinkedomParser {parseFromString(html,type){return super.parseFromString('<!doctype html><html><body>'+html+'</body></html>',type)}}
const start = lines.findIndex(x => x.startsWith('async function compose('));
const end = lines.findIndex((x, i) => i > start && x.startsWith('function parseRecipients('));
class Element {
  constructor(tag, cls, text) { this.tag=tag; this.className=cls; this.textContent=text; this.children=[]; this.value=''; }
  setAttribute(k,v) { this[k]=v; }
  append(...items) { this.children.push(...items); }
  replaceChildren(...items) { this.children=items; }
  focus() {}
  remove() {}
  click() {}
}
function context(original) {
  const messages=[];
  const calls=[];
  const c={ console, Date, Promise, Error, setTimeout, clearTimeout, DOMParser,
    draft:null, draftTimer:null, status:{oidc:true}, user:{mailAddress:'qa@example.test'},
    identities:[{id:'qa-id',email:'qa@example.test'}], account:'synthetic',
    el:(...args)=>new Element(...args),
    button:(label, fn)=>{const b=new Element('button');b.label=label;b.onclick=fn;return b;},
    document:{body:new Element('body')}, toast:x=>messages.push(x),error:e=>messages.push(e.message),
    reloadFolders:async()=>{},loadMessages:async()=>{},folder:()=>({id:'drafts'}),
    ensureFolder:async()=>({id:'synthetic-folder'}),
    jmap:async(method,args)=>{calls.push({method,args});if(method==='Email/get')return {list:[original]};return {created:{draft:{id:'synthetic-draft'},send:{id:'synthetic-submission'}}};},
  };
  vm.createContext(c);
  vm.runInContext(readable+'\n'+lines.slice(start,end+1).join('\n'),c);
  return {c,messages,calls};
}
const flatten = e => [e,...e.children.flatMap(flatten)];
(async()=>{
  const results=[];
  const attachment={blobId:'synthetic-blob',name:'QA attachment.txt',type:'text/plain'};
  const message={subject:'Workspace QA',receivedAt:'2026-10-15T12:00:00Z',from:[{email:'sender@example.test'}],textBody:[{partId:'text'}],bodyValues:{text:{value:'Synthetic message body'}},attachments:[attachment]};
  {
    const {c}=context(); await c.compose(null,message,true);
    results.push({check:'Forward retains attachment',expected:1,actual:c.draft.files.length,issue:c.draft.files.length!==1});
  }
  {
    const {c}=context(message); await c.compose('draft-id');
    const list=flatten(c.document.body).find(e=>e.className==='attachment-list');
    results.push({check:'Reopened draft renders existing attachment',expected:1,actual:list.children.length,retainedAttachmentCount:c.draft.files.length,issue:list.children.length!==1});
  }
  {
    const htmlMessage={...message,textBody:[],htmlBody:[{partId:'html'}],bodyValues:{html:{value:'<p>Synthetic HTML message</p>'}}};
    const {c}=context(); await c.compose(null,htmlMessage,true);
    results.push({check:'Forward includes HTML-only source message text',expected:true,actual:c.draft.body.value.includes('Synthetic HTML message'),issue:!c.draft.body.value.includes('Synthetic HTML message')});
  }
  {
    const {c,messages,calls}=context(); await c.compose();
    c.draft.fields.cc.value='recipient@example.test';c.draft.body.value='Synthetic CC-only test';
    const form=flatten(c.document.body).find(e=>e.tag==='form');
    await form.onsubmit({preventDefault(){}});
    results.push({check:'Valid CC-only email can be submitted',expected:'submission attempted',actual:messages[0],apiCalls:calls.length,issue:calls.length===0});
  }
  console.log(JSON.stringify(results,null,2)); require('node:assert/strict').equal(results.filter(r=>r.issue).length,0);
})().catch(e=>{console.error(e);process.exitCode=1;});
