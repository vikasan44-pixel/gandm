CREATE TABLE permission_set_tools (
    set_id  uuid NOT NULL REFERENCES permission_sets (id) ON DELETE CASCADE,
    tool_id uuid NOT NULL REFERENCES tools (id) ON DELETE CASCADE,
    PRIMARY KEY (set_id, tool_id)
);

CREATE INDEX idx_permission_set_tools_tool_id ON permission_set_tools (tool_id);
