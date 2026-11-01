"""Trusted adapter only: no formula-to-code conversion, macros or link updates."""
import json
import subprocess
import sys
import time
import uno
from com.sun.star.beans import PropertyValue
from com.sun.star.document.MacroExecMode import NEVER_EXECUTE
from com.sun.star.document.UpdateDocMode import NO_UPDATE

version = subprocess.check_output(["/usr/bin/soffice", "--version"], text=True).strip()
if sys.argv[1:] == ["--version"]:
    print(json.dumps({"provider": "libreoffice-isolated", "version": version}))
    sys.exit(0)

def prop(name, value):
    item = PropertyValue()
    item.Name, item.Value = name, value
    return item

process = subprocess.Popen([
    "/usr/bin/soffice", "--headless", "--nologo", "--nodefault", "--norestore",
    "-env:UserInstallation=file:///tmp/keel-profile",
    "--accept=pipe,name=keelcalc;urp;StarOffice.ComponentContext",
], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
document = None
try:
    local = uno.getComponentContext()
    resolver = local.ServiceManager.createInstanceWithContext("com.sun.star.bridge.UnoUrlResolver", local)
    remote = None
    for _ in range(100):
        try:
            remote = resolver.resolve("uno:pipe,name=keelcalc;urp;StarOffice.ComponentContext")
            break
        except Exception:
            time.sleep(.1)
    if remote is None:
        raise RuntimeError("CALC_START_FAILED")
    desktop = remote.ServiceManager.createInstanceWithContext("com.sun.star.frame.Desktop", remote)
    document = desktop.loadComponentFromURL("file:///input/source.xlsx", "_blank", 0, (
        prop("Hidden", True), prop("ReadOnly", True),
        prop("MacroExecutionMode", NEVER_EXECUTE), prop("UpdateDocMode", NO_UPDATE),
    ))
    if document is None:
        raise RuntimeError("CALC_OPEN_FAILED")
    document.calculateAll()
    document.storeToURL("file:///output/recalculated.xlsx", (
        prop("FilterName", "Calc MS Excel 2007 XML"), prop("Overwrite", False),
    ))
    print(json.dumps({"provider": "libreoffice-isolated", "version": version, "status": "completed"}))
except Exception:
    # Do not print UNO messages: they may contain source content or file paths.
    print(json.dumps({"status": "failed", "error_code": "CALC_EXECUTION_FAILED"}))
    sys.exit(1)
finally:
    if document is not None:
        document.close(True)
    process.terminate()
    try:
        process.wait(timeout=5)
    except subprocess.TimeoutExpired:
        process.kill()
