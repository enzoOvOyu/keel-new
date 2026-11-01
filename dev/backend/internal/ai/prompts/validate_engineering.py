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
    kind: Literal['equipment','assembly','site','room','rack','placement','port','connection','cable','power','material','workload','other']
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
class Quantity(Closed):
    value: str
    unit: str
class TypedValue(Closed):
    kind: Literal['decimal','text','boolean','reference']
    text: str | None = None
    decimal: str | None = None
    boolean: bool | None = None
    reference: str | None = None
    unit: str | None = None
class NamedValue(Closed):
    name: str
    value: TypedValue
class RuleIO(Closed):
    name: str
    value_type: str
    unit: str
    required: bool
class Expression(Closed):
    operator: Literal['input','literal','add','subtract','multiply','divide','min','max','ceil','floor','abs','select_ceiling','outputs','output']
    input: str | None = None
    literal: Quantity | None = None
    arguments: list['Expression']
class VersionReference(Closed):
    id: str
    version: int = Field(ge=1)
class RuleCase(Closed):
    id: str
    inputs: list[NamedValue]
    expected: list[NamedValue]
    tolerance: str
class Rule(Closed):
    id: str
    version: int = Field(ge=1)
    domain: Literal['source','equipment','facility_placement','power_cooling','connectivity_cabling','material_workload','network_design','validation_release']
    scope_type: Literal['project','customer','domain']
    scope_id: str
    inputs: list[RuleIO]
    outputs: list[RuleIO]
    expression: Expression
    dependencies: list[VersionReference]
    validation_cases: list[RuleCase]
    approval_status: Literal['draft']
class FormulaCell(Closed):
    sheet_id: str
    address: str
class Action(Closed):
    action: Literal['inspect_workbook','inspect_sheet','inspect_region','read_range','inspect_formula','trace_formula_dependencies','classify_information','map_semantic_fields','resolve_cross_sheet_references','propose_engineering_model','propose_calculation_rule','run_calculation','validate_model','analyze_validation_issues']
    sheet: str | None = None
    bounds: Bounds | None = None
    cell: str | None = None
    proposal: Proposal | None = None
    rule: Rule | None = None
    rule_ref: VersionReference | None = None
    cells: list[FormulaCell] | None = Field(default=None, max_length=32)
    inputs: list[NamedValue] | None = None
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
        if self.action == "propose_calculation_rule" and self.rule is None:
            raise ValueError("rule required")
        if self.action == "run_calculation" and self.rule_ref is None and not self.cells:
            raise ValueError("formula cells or approved rule required")
        return self
try:
    Action.model_validate_json(sys.stdin.buffer.read())
except Exception as error:
    if hasattr(error, 'errors'):
        print('; '.join(str(item['loc'])+':'+item['type'] for item in error.errors(include_input=False))[:1200])
    sys.exit(1)
