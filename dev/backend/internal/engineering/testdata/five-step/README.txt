Synthetic five-step standard-input workbooks

Every identifier, MAC, address, device name and engineering quantity is synthetic.
No customer workbook or credentials were copied into these fixtures.

Generate from dev/backend:
  go run ./cmd/engineering-fixtures
Run native XLSX and independent model expectations:
  go test ./internal/engineering -run 'Test(FiveStepXLSX|StandardXLSX)'

expected.json contains independently stated engineering expectations. It is not
an acceptance report. Rules/policy references must be explicitly installed and
reviewed in an isolated test project; fixture names do not grant approval.

Current scope: all five files pass format validation; native model expectations
cover steps 1-2. Step 3 routing kernel has independent unit cases, but complete
cabling source import and calculation publication are not implemented. Steps
4-5 standard model import and the new scoped Agent are not implemented.
See docs/intelligent-deployment/FIXED-STEP-XLSX-TEST-20261101.json for actual
isolated API/database/publication/topology results. No live device verification.
