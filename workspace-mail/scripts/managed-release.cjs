// Generate the explicit managed-update contract from a verified suite catalog.
// Usage: node managed-release.cjs CATALOG RELEASE_DIRECTORY VERSION
const fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto');
const [catalogFile,releaseDir,version]=process.argv.slice(2);
if(!catalogFile||!releaseDir||!/^\d+\.\d+\.\d+-preview\.\d+$/.test(version||''))throw Error('Provide catalog, release directory and preview version');
const catalog=JSON.parse(fs.readFileSync(catalogFile,'utf8'));
const app=structuredClone(catalog.apps.find(a=>a.id==='mail'));
if(!app||app.package!=='mail'||app.executable!=='Mail.exe'||app.env?.WORKSPACE_MAIL_BACKEND!=='private')throw Error('Expected the compatible private Mail integration');
const repository='oneworkspacestudio/mail',tag='workspace-mail-v'+version;
app.version=version;app.source='https://github.com/'+repository+'/releases/tag/'+tag;
const packages={};
for(const [id,asset] of [['mail','Mail.exe'],['mail-license','LICENSE.txt'],['mail-notice','NOTICE.md']]){
 const previous=catalog.packages[id];if(!previous)throw Error('Catalog lacks '+id);
 const bytes=fs.readFileSync(path.join(releaseDir,asset));
 packages[id]={file:previous.file,size:bytes.length,sha256:crypto.createHash('sha256').update(bytes).digest('hex'),asset};
}
fs.writeFileSync(path.join(releaseDir,'workspace-update.json'),JSON.stringify({schema:1,integration:2,repository,tag,apps:[app],packages},null,2)+'\n');
