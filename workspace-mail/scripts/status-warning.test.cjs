const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const path=require('node:path');
const source=fs.readFileSync(path.join(__dirname,'../assets/app.js'),'utf8');
const fn=source.split(/\r?\n/).find(line=>line.startsWith('function updateIncomingWarning()'));
assert.ok(fn,'incoming status renderer exists');
for(const state of ['attention','unverified','reachable','checking']){
 const shown=[];
 const context={status:{inboundState:state},root:{querySelector(){return {querySelector(){return null},prepend(x){shown.push(x)}}}},hostingPage(){},button(label){return label},el(tag,cls,text){return {text,append(){}}}};
 vm.runInNewContext(fn+';updateIncomingWarning()',context,{timeout:1000});
 assert.equal(shown.length,['attention','unverified'].includes(state)?1:0);
 if(state==='unverified'){
  assert.match(shown[0].text,/check is unavailable or inconclusive/);
  assert.match(shown[0].text,/may still be receiving/);
  assert.doesNotMatch(shown[0].text,/Incoming email is currently unavailable/);
 }
 if(state==='attention')assert.match(shown[0].text,/SMTP could not be reached/);
}
console.log('Incoming status wording checks passed');