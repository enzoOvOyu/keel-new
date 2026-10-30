"""Controlled JSON contract validator; no files, network or asset writes."""
import json
import sys
from typing import Literal
from pydantic import BaseModel, ConfigDict, Field, ValidationError

class Extractor(BaseModel):
    model_config = ConfigDict(extra='forbid')
    key: str = ''
    entity_type: str
    record_kind: Literal['planned_device','material_summary','component','physical_connection','physical_port','logical_connection','reference']
    start_row: int = Field(ge=1)
    end_row: int = Field(ge=1)
    fields: dict[str, int]
    type_column: int = 0
    type_values: dict[str, str] = {}
    row_keys: list[str] = []
    confidence: float = Field(ge=0, le=1)

class ExtractionPlan(BaseModel):
    model_config = ConfigDict(extra='forbid')
    purpose: str
    header_rows: list[int]
    tables: list[Extractor]
    note: str

try:
    ExtractionPlan.model_validate_json(sys.stdin.read())
    print('{"valid":true}')
except ValidationError as exc:
    # Locations and error codes only: never echo source material into diagnostics.
    print(json.dumps({'valid':False,'errors':[{'location':list(e['loc']),'type':e['type']} for e in exc.errors(include_input=False)]}))
    sys.exit(1)
