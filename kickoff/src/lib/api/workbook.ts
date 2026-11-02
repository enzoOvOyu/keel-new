import { unzipSync, zipSync, strFromU8, strToU8 } from 'fflate';
import type { ImportPreview, ModelRow, TableDefinition } from './types';

const xml = (s: string) => s.replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&quot;');
const column = (n: number): string => n<26?String.fromCharCode(65+n):column(Math.floor(n/26)-1)+String.fromCharCode(65+n%26);
function parseCSV(text: string): string[][] {
  const rows: string[][]=[]; let row: string[]=[], value='', quoted=false;
  const input=text.replace(/^\uFEFF/,'');
  for(let i=0;i<input.length;i++) {
    const c=input[i];
    if(c==='"') { if(quoted&&input[i+1]==='"'){value+='"';i++;}else quoted=!quoted; }
    else if(c===','&&!quoted){row.push(value);value='';}
    else if((c==='\n'||c==='\r')&&!quoted){if(c==='\r'&&input[i+1]==='\n')i++;row.push(value);if(row.some(v=>v.trim()))rows.push(row);row=[];value='';}
    else value+=c;
  }
  if(quoted)throw Error('CSV引号不完整。');
  row.push(value);if(row.some(v=>v.trim()))rows.push(row);
  return rows;
}
export async function readModelFile(file: File, table: TableDefinition): Promise<ImportPreview> {
  let data: string[][]=[],sheet='CSV';
  if(/\.csv$/i.test(file.name)) data=parseCSV(await file.text());
  else if(/\.xlsx$/i.test(file.name)) {
    const archive=unzipSync(new Uint8Array(await file.arrayBuffer()));
    const parse=(path:string)=>{
      if(!archive[path])throw Error('工作簿缺少必要结构：'+path);
      const doc=new DOMParser().parseFromString(strFromU8(archive[path]),'application/xml');
      if(doc.querySelector('parsererror'))throw Error('工作簿 XML 无法读取。');return doc;
    };
    const book=parse('xl/workbook.xml');
    const sheets=[...book.getElementsByTagName('sheet')];
    const selected=sheets.find(s=>s.getAttribute('name')===table.id||s.getAttribute('name')===table.name)??(sheets.length===1?sheets[0]:undefined);
    if(!selected)throw Error(`请提供单表工作簿，或将目标表命名为「${table.name}」。非标准资料请从资料入口添加。`);
    sheet=selected.getAttribute('name')??table.name;
    const relId=selected.getAttribute('r:id');
    const rel=[...parse('xl/_rels/workbook.xml.rels').getElementsByTagName('Relationship')].find(r=>r.getAttribute('Id')===relId);
    const target=rel?.getAttribute('Target');if(!target)throw Error('无法定位工作表。');
    const path=target.startsWith('/')?target.slice(1):'xl/'+target.replace(/^\.\//,'');
    const shared=archive['xl/sharedStrings.xml']?[...parse('xl/sharedStrings.xml').getElementsByTagName('si')].map(si=>[...si.getElementsByTagName('t')].map(t=>t.textContent??'').join('')):[];
    for(const r of [...parse(path).getElementsByTagName('row')]) {
      const values:string[]=[];
      for(const c of [...r.getElementsByTagName('c')]) {
        if(c.getElementsByTagName('f').length)throw Error('建模输入含公式，请提供已确认的数值。公式资料可从资料入口提交分析。');
        const ref=c.getAttribute('r')??'A1';let index=0;for(const char of ref.replace(/[\d]/g,''))index=index*26+char.charCodeAt(0)-64;
        const value=c.getElementsByTagName('v')[0]?.textContent??'';
        values[index-1]=c.getAttribute('t')==='s'?shared[Number(value)]??'':c.getAttribute('t')==='inlineStr'?[...c.getElementsByTagName('t')].map(t=>t.textContent??'').join(''):value;
      }
      if(values.some(v=>v?.trim()))data.push(values);
    }
  } else throw Error('标准建模输入支持 XLSX 或 CSV。');
  if(!data.length)throw Error('文件没有可读取的数据。');
  const headers=data[0].map(v=>v?.trim()??'');
  const unknownHeaders=headers.filter(h=>h&&!table.fields.some(f=>f.key===h||f.label===h));
  const errors=unknownHeaders.length?[`未知列：${unknownHeaders.join('、')}。请核对表头后重新导入，避免遗漏。`]:[];
  const rows=data.slice(1).map(values=>Object.fromEntries(table.fields.map(f=>{
    const index=headers.findIndex(h=>h===f.key||h===f.label);
    return [f.key,index>=0?values[index]?.trim()??'':''];
  })));
  if(!headers.some(h=>table.fields.some(f=>f.key===h||f.label===h)))errors.push('没有匹配当前建模表的字段。');
  return {rows,errors,unknownHeaders,sheet};
}
export function downloadFile(content: BlobPart, name: string, type='text/plain;charset=utf-8') {
  const url=URL.createObjectURL(new Blob([content],{type}));
  const a=document.createElement('a');a.href=url;a.download=name;a.click();setTimeout(()=>URL.revokeObjectURL(url),2000);
}
export function exportWorkbook(table: TableDefinition, rows: ModelRow[], template=false) {
  const data=[table.fields.map(f=>f.key),...rows.map(r=>table.fields.map(f=>r[f.key]??''))];
  const worksheet=`<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>${data.map((r,i)=>`<row r="${i+1}">${r.map((v,j)=>`<c r="${column(j)}${i+1}" t="inlineStr"><is><t xml:space="preserve">${xml(v)}</t></is></c>`).join('')}</row>`).join('')}</sheetData></worksheet>`;
  const files: Record<string,Uint8Array>={
    '[Content_Types].xml':strToU8('<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>'),
    '_rels/.rels':strToU8('<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>'),
    'xl/workbook.xml':strToU8(`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="${xml(table.id)}" sheetId="1" r:id="rId1"/></sheets></workbook>`),
    'xl/_rels/workbook.xml.rels':strToU8('<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>'),
    'xl/worksheets/sheet1.xml':strToU8(worksheet),
  };
  downloadFile(zipSync(files).buffer as ArrayBuffer,`${table.name}${template?'-模板':''}.xlsx`,'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet');
}
