CREATE TABLE user_tools (
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    tool_id uuid NOT NULL REFERENCES tools (id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, tool_id)
);

CREATE INDEX idx_user_tools_tool_id ON user_tools (tool_id);
