-- Universal Table Compatibility Processing System
-- Supports arbitrary table and field additions, implements intelligent table structure management

-- ============================================================================
-- Core Utility Functions
-- ============================================================================

-- Function to check if table exists
DELIMITER $$
DROP PROCEDURE IF EXISTS CheckTableExists$$
CREATE PROCEDURE CheckTableExists(
    IN p_table_name VARCHAR(64),
    OUT table_exists BOOLEAN
)
BEGIN
    DECLARE table_count INT DEFAULT 0;

    SELECT COUNT(*) INTO table_count
    FROM information_schema.tables
    WHERE table_schema = DATABASE()
    AND table_name = p_table_name;

    SET table_exists = (table_count > 0);
END$$
DELIMITER ;

-- Function to check if column exists
DELIMITER $$
DROP PROCEDURE IF EXISTS CheckColumnExists$$
CREATE PROCEDURE CheckColumnExists(
    IN p_table_name VARCHAR(64),
    IN p_column_name VARCHAR(64),
    OUT column_exists BOOLEAN
)
BEGIN
    DECLARE column_count INT DEFAULT 0;

    SELECT COUNT(*) INTO column_count
    FROM information_schema.columns
    WHERE table_schema = DATABASE()
    AND table_name = p_table_name
    AND column_name = p_column_name;

    SET column_exists = (column_count > 0);
END$$
DELIMITER ;

-- ============================================================================
-- Universal Table Structure Management Functions
-- ============================================================================

-- Universal table creation and field addition function
-- Parameter description:
-- table_name: table name
-- create_table_sql: complete CREATE TABLE SQL statement (should include all desired columns)
-- columns_to_add: comma-separated list of column names to add (e.g., 'ext_info,new_column')
-- Note: This procedure will create table if not exists, or add specified columns if table exists
DELIMITER $$
DROP PROCEDURE IF EXISTS CozeLoopTableManager$$
CREATE PROCEDURE CozeLoopTableManager(
    IN table_name VARCHAR(64),
    IN create_table_sql TEXT,
    IN columns_to_add TEXT
)
BEGIN
    DECLARE table_exists BOOLEAN DEFAULT FALSE;
    DECLARE i INT DEFAULT 0;
    DECLARE column_count INT DEFAULT 0;
    DECLARE column_name VARCHAR(255);
    DECLARE column_definition TEXT;
    DECLARE column_exists BOOLEAN DEFAULT FALSE;
    DECLARE sql_stmt TEXT;
    DECLARE columns_array JSON DEFAULT NULL;
    DECLARE current_column VARCHAR(255);
    DECLARE current_definition TEXT;

    -- Parse columns_to_add parameter into JSON array
    IF columns_to_add IS NULL OR columns_to_add = '' THEN
        SET columns_array = JSON_ARRAY();
    ELSE
        -- Convert comma-separated string to JSON array
        SET columns_array = JSON_ARRAY();
        SET @temp_str = columns_to_add;
        SET @pos = 1;

        WHILE @pos <= LENGTH(@temp_str) DO
            SET @comma_pos = LOCATE(',', @temp_str, @pos);
            IF @comma_pos = 0 THEN
                -- Last column
                SET current_column = TRIM(SUBSTRING(@temp_str, @pos));
                IF current_column != '' THEN
                    SET columns_array = JSON_ARRAY_APPEND(columns_array, '$', current_column);
                END IF;
                SET @pos = LENGTH(@temp_str) + 1;
            ELSE
                -- Extract column name before comma
                SET current_column = TRIM(SUBSTRING(@temp_str, @pos, @comma_pos - @pos));
                IF current_column != '' THEN
                    SET columns_array = JSON_ARRAY_APPEND(columns_array, '$', current_column);
                END IF;
                SET @pos = @comma_pos + 1;
            END IF;
        END WHILE;
    END IF;

    -- Extract column definitions for specified columns from CREATE TABLE statement
    SET @create_sql = create_table_sql;
    SET @left_paren = LOCATE('(', @create_sql);
    SET @pos = @left_paren + 1;
    SET @paren_count = 1;

    -- Find the matching right parenthesis
    WHILE @pos <= LENGTH(@create_sql) AND @paren_count > 0 DO
        SET @char = SUBSTRING(@create_sql, @pos, 1);
        IF @char = '(' THEN
            SET @paren_count = @paren_count + 1;
        ELSEIF @char = ')' THEN
            SET @paren_count = @paren_count - 1;
        END IF;
        SET @pos = @pos + 1;
    END WHILE;
    SET @right_paren = @pos - 1;

    -- Extract table definition between parentheses
    SET @create_sql = SUBSTRING(@create_sql, @left_paren + 1, @right_paren - @left_paren - 1);

    -- Check if table exists
    CALL CheckTableExists(table_name, table_exists);

    -- If table doesn't exist, create it
    IF NOT table_exists THEN
        SET @sql = create_table_sql;
        PREPARE stmt FROM @sql;
        EXECUTE stmt;
        DEALLOCATE PREPARE stmt;

        SELECT CONCAT('Table ', table_name, ' created successfully') as result;
    ELSE

        -- When table exists, add specified columns if they don't exist
        IF columns_array IS NOT NULL AND JSON_LENGTH(columns_array) > 0 THEN
            SET column_count = JSON_LENGTH(columns_array);
            SET i = 0;

            WHILE i < column_count DO
                -- Get current column name
                SET current_column = JSON_UNQUOTE(JSON_EXTRACT(columns_array, CONCAT('$[', i, ']')));

                -- Check if column already exists
                CALL CheckColumnExists(table_name, current_column, column_exists);

                IF NOT column_exists THEN
                    -- Find column definition in CREATE TABLE statement
                    SET @search_pattern = CONCAT('`', current_column, '`');
                    SET @col_start = LOCATE(@search_pattern, @create_sql);

                    IF @col_start > 0 THEN
                        -- Find the end of this column definition (next comma or constraint)
                        SET @next_comma = LOCATE(',', @create_sql, @col_start);
                        SET @next_constraint = LEAST(
                            IFNULL(NULLIF(LOCATE('PRIMARY KEY', @create_sql, @col_start), 0), LENGTH(@create_sql) + 1),
                            IFNULL(NULLIF(LOCATE('UNIQUE KEY', @create_sql, @col_start), 0), LENGTH(@create_sql) + 1),
                            IFNULL(NULLIF(LOCATE('KEY', @create_sql, @col_start), 0), LENGTH(@create_sql) + 1),
                            IFNULL(NULLIF(LOCATE('INDEX', @create_sql, @col_start), 0), LENGTH(@create_sql) + 1),
                            IFNULL(NULLIF(LOCATE('CONSTRAINT', @create_sql, @col_start), 0), LENGTH(@create_sql) + 1),
                            IFNULL(NULLIF(LOCATE('FOREIGN KEY', @create_sql, @col_start), 0), LENGTH(@create_sql) + 1)
                        );

                        -- Determine where column definition ends
                        SET @col_end = LENGTH(@create_sql) + 1;
                        IF @next_comma > 0 AND @next_comma < @col_end THEN
                            SET @col_end = @next_comma;
                        END IF;
                        IF @next_constraint > 0 AND @next_constraint < @col_end THEN
                            SET @col_end = @next_constraint;
                        END IF;

                        -- Extract column definition (everything after the column name)
                        SET current_definition = TRIM(SUBSTRING(@create_sql, @col_start + LENGTH(@search_pattern), @col_end - @col_start - LENGTH(@search_pattern)));

                        -- Validate column definition
                        IF current_definition = '' OR current_definition IS NULL THEN
                            SELECT CONCAT('ERROR: Empty column definition for ', current_column, ' in table ', table_name) as result;
                        ELSE
                            -- Log column definition for debugging (only when adding column)
                            SELECT CONCAT('Adding column: ', current_column, ' with definition: ', current_definition) as debug;

                            -- Build and execute ALTER statement
                            SET sql_stmt = CONCAT('ALTER TABLE `', table_name, '` ADD COLUMN `', current_column, '` ', current_definition);
                            SET @sql = sql_stmt;
                            PREPARE stmt FROM @sql;
                            EXECUTE stmt;
                            DEALLOCATE PREPARE stmt;

                            SELECT CONCAT('Column ', current_column, ' added to table ', table_name) as result;
                        END IF;
                    ELSE
                        SELECT CONCAT('ERROR: Column ', current_column, ' not found in CREATE TABLE statement') as result;
                    END IF;
                ELSE
                    SELECT CONCAT('Column ', current_column, ' already exists in table ', table_name) as result;
                END IF;

                SET i = i + 1;
            END WHILE;
        ELSE
            SELECT CONCAT('No columns specified for addition') as result;
        END IF;
    END IF;
END$$
DELIMITER ;

-- Simplified field addition function (recommended)
-- Parameter description:
-- table_name: table name
-- column_name: new field name
-- column_definition: field definition (e.g., VARCHAR(255) COMMENT 'field description')
-- after_column: after which field to add (optional, NULL means add to the end)
DELIMITER $$
DROP PROCEDURE IF EXISTS SafeAddColumn$$
CREATE PROCEDURE SafeAddColumn(
    IN p_table_name VARCHAR(64),
    IN p_column_name VARCHAR(64),
    IN p_column_definition TEXT,
    IN p_after_column VARCHAR(64)
)
BEGIN
    DECLARE table_exists BOOLEAN DEFAULT FALSE;
    DECLARE column_exists BOOLEAN DEFAULT FALSE;
    DECLARE sql_stmt TEXT;

    -- Check if table exists
    CALL CheckTableExists(p_table_name, table_exists);

    IF table_exists THEN
        -- Check if field exists
        CALL CheckColumnExists(p_table_name, p_column_name, column_exists);

        IF NOT column_exists THEN
            -- Build ALTER statement
            IF p_after_column IS NULL OR p_after_column = '' THEN
                SET sql_stmt = CONCAT('ALTER TABLE `', p_table_name, '` ADD COLUMN `', p_column_name, '` ', p_column_definition);
            ELSE
                SET sql_stmt = CONCAT('ALTER TABLE `', p_table_name, '` ADD COLUMN `', p_column_name, '` ', p_column_definition, ' AFTER `', p_after_column, '`');
            END IF;

            -- Execute ALTER statement
            SET @sql = sql_stmt;
            PREPARE stmt FROM @sql;
            EXECUTE stmt;
            DEALLOCATE PREPARE stmt;

            SELECT CONCAT('Column ', p_column_name, ' added to table ', p_table_name) as result;
        ELSE
            SELECT CONCAT('Column ', p_column_name, ' already exists in table ', p_table_name) as result;
        END IF;
    ELSE
        SELECT CONCAT('Table ', p_table_name, ' does not exist') as result;
    END IF;
END$$
DELIMITER ;

-- ============================================================================
-- Usage Examples and Instructions
-- ============================================================================

/*
Usage examples:

1. Using SafeAddColumn to add a single field:
   CALL SafeAddColumn('user', 'new_field', 'VARCHAR(255) COMMENT ''New field''', 'id');

2. Using CozeLoopTableManager for table management with specified columns:
   -- Create new table
   CALL CozeLoopTableManager(
       'new_table',
       'CREATE TABLE new_table (id INT PRIMARY KEY, name VARCHAR(100), created_at DATETIME DEFAULT CURRENT_TIMESTAMP)',
       NULL
   );

   -- Add specific columns to existing table
   CALL CozeLoopTableManager(
       'existing_table',
       'CREATE TABLE existing_table (id INT, name VARCHAR(100), email VARCHAR(255), created_at DATETIME)',
       'email,created_at'
   );

   -- Add single column
   CALL CozeLoopTableManager(
       'prompt_commit',
       'CREATE TABLE prompt_commit (..., ext_info text COLLATE utf8mb4_general_ci COMMENT ''扩展字段'', ...)',
       'ext_info'
   );
*/
