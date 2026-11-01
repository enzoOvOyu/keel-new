import sys
from typing import Literal
from pydantic import BaseModel, ConfigDict, Field, model_validator

class Closed(BaseModel):
    model_config = ConfigDict(extra='forbid', strict=True)
class Bounds(Closed):
    start_row: int = Field(ge=1)
    end_row: int = Field(ge=1)
    start_column: int = Field(ge=1)
    end_column: int = Field(ge=1)
class Column(Closed):
    field: str
    column: int = Field(ge=1)
    unit: str
class Constant(Closed):
    field: str
    value: str
    evidence_cells: list[str] = Field(min_length=1)
class Region(Closed):
    sheet: str
    bounds: Bounds
    kind: Literal['equipment','assembly','site','room','rack','placement','other']
    semantic: str
    header_rows: list[int]
    columns: list[Column]
    constants: list[Constant]
class Module(Closed):
    module_id: str
    applicable: bool
    reason: str = Field(min_length=1)
    sheets: list[str]
class Proposal(Closed):
    summary: str = Field(min_length=1)
    regions: list[Region] = Field(min_length=1)
    modules: list[Module]
    questions: list[str]
class Action(Closed):
    action: Literal['inspect_workbook','inspect_sheet','inspect_region','read_range','inspect_formula','trace_formula_dependencies','classify_information','map_semantic_fields','resolve_cross_sheet_references','propose_engineering_model','propose_calculation_rule','run_calculation','validate_model','analyze_validation_issues']
    sheet: str | None = None
    bounds: Bounds | None = None
    cell: str | None = None
    proposal: Proposal | None = None
    @model_validator(mode='after')
    def arguments(self):
        if self.action in ('inspect_sheet','inspect_region','read_range','inspect_formula','trace_formula_dependencies','resolve_cross_sheet_references') and not self.sheet:
            raise ValueError('sheet required')
        if self.action in ('inspect_region','read_range') and self.bounds is None:
            raise ValueError('bounds required')
        if self.action == 'inspect_formula' and not self.cell:
            raise ValueError('cell required')
        if self.action in ('classify_information','map_semantic_fields','propose_engineering_model','validate_model','analyze_validation_issues') and self.proposal is None:
            raise ValueError('proposal required')
        return self
try:
    Action.model_validate_json(sys.stdin.buffer.read())
except Exception as error:
    if hasattr(error, 'errors'):
        print('; '.join(str(item['loc'])+':'+item['type'] for item in error.errors(include_input=False))[:1200])
    sys.exit(1)
