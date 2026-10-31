"""Copy only native component dependencies from an explicitly supplied pinned checkout."""
from pathlib import Path
import sys,re,shutil,subprocess
checkout=Path(sys.argv[1]).resolve()
revision=subprocess.check_output(['git','rev-parse','HEAD'],cwd=checkout,text=True).strip()
if revision!='49b62a2c820d37e98a1c7b63aa35fd83f960d4ec':raise SystemExit('Upstream revision does not match pinned source')
source=checkout/'ui/packages/design-system/src';target=Path(__file__).resolve().parents[1]/'src/vendor/kestra/design-system';visited=set()
def visit(p):
 p=p.resolve()
 if p in visited:return
 visited.add(p);dest=target/p.relative_to(source);dest.parent.mkdir(parents=True,exist_ok=True)
 if not dest.exists():shutil.copyfile(p,dest)
 if p.suffix not in ['.vue','.ts','.scss']:return
 for ref in re.findall(r'''(?:from\s*|import\s*|@use\s*|@forward\s*)["']([^"']+)["']''',p.read_text()):
  if not(ref.startswith('.') or (p.suffix=='.scss' and '/' not in ref)):continue
  t=p.parent/ref
  for q in [t,t.with_suffix('.ts'),t.with_suffix('.vue'),t.with_suffix('.scss'),t.parent/('_'+t.name+'.scss'),t/'index.ts']:
   if q.is_file():visit(q);break
for arg in sys.argv[2:]:visit(source/arg)
print(f'Copied/retained {len(visited)} source dependencies from {revision}')
