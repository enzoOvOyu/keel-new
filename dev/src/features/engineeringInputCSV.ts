// Preserve quoted cells and embedded line breaks while editing a standard input table.
export function readInputCSV(text:string):string[][]|null {
 if(!text.trim())return [];
 const rows:string[][]=[];let row:string[]=[],value='',quoted=false;
 const source=text.replace(/^\uFEFF/,'').replace(/\r\n/g,'\n');
 for(let i=0;i<source.length;i++){
  const c=source[i];
  if(c==='"'){if(quoted&&source[i+1]==='"'){value+='"';i++}else quoted=!quoted}
  else if(!quoted&&(c===','||c==='\n')){row.push(value);value='';if(c==='\n'){rows.push(row);row=[]}}
  else value+=c;
 }
 if(quoted)return null;
 if(value||row.length){row.push(value);rows.push(row)}
 return rows;
}
export function writeInputCSV(rows:string[][]):string {
 return rows.map(row=>row.map(value=>/[",\r\n]/.test(value)?'"'+value.replaceAll('"','""')+'"':value).join(',')).join('\r\n');
}
