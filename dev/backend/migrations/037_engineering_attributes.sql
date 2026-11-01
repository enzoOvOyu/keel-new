CREATE TABLE eng_attribute_definition (
 project_id text NOT NULL REFERENCES project(id) ON DELETE CASCADE,id uuid NOT NULL,version integer NOT NULL,
 entity_kind text NOT NULL,key text NOT NULL,value_type text NOT NULL CHECK(value_type IN('text','decimal','boolean','reference')),
 unit text,enum_values text[] NOT NULL,required boolean NOT NULL,approval_status text NOT NULL CHECK(approval_status IN('draft','approved','retired')),
 created_by text NOT NULL,reviewed_by text,reason text NOT NULL,created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(project_id,id,version),UNIQUE(project_id,entity_kind,key,version)
);
CREATE TABLE eng_attribute_value (
 revision_id uuid NOT NULL,entity_id uuid NOT NULL,project_id text NOT NULL,definition_id uuid NOT NULL,definition_version integer NOT NULL,
 value_kind text NOT NULL,text_value text,decimal_value numeric,boolean_value boolean,reference_value text,unit text,
 PRIMARY KEY(revision_id,entity_id,definition_id),
 FOREIGN KEY(revision_id,entity_id) REFERENCES eng_entity_version(revision_id,entity_id) ON DELETE CASCADE,
 FOREIGN KEY(project_id,definition_id,definition_version) REFERENCES eng_attribute_definition(project_id,id,version),
 CHECK(num_nonnulls(text_value,decimal_value,boolean_value,reference_value)=1)
);
INSERT INTO schema_migration(version) VALUES(37);
