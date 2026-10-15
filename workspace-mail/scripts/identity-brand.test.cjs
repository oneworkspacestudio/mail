const assert=require('node:assert/strict');
const fs=require('node:fs');const path=require('node:path');const vm=require('node:vm');
const source=fs.readFileSync(path.join(__dirname,'../assets/app.js'),'utf8');
const fn=source.split(/\r?\n/).find(line=>line.startsWith('function brand()'));
for(const base of ['', '/mail']){
 const context={base,el(tag,cls,text){return {tag,cls,text,children:[],append(...children){this.children.push(...children)}}}};
 const mark=vm.runInNewContext(fn+';brand()',context,{timeout:1000});
 assert.equal(mark.children[0].src,base+'/assets/one-workspace.svg');
 assert.equal(mark.children[0].alt,'One Workspace');
 assert.equal(mark.children[1].children[1].text,'One Workspace');
 assert.doesNotMatch(fn,/#ea4335|#34a853|#fbbc04/);
}
assert.match(source,/Independent, self-hosted One Workspace Mail\. Not affiliated with Google\./);
assert.match(source,/Account server: /);
console.log('Mail identity checks passed');