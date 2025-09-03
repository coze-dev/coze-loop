-- ============================================================================
-- Universal ALTER TABLE DDL Parser and Compatibility Checker
-- ============================================================================

-- Function to check if table exists
DELIMITER $$
DROP PROCEDURE IF EXISTS CozeLoopCheckTableExists$$
CREATE PROCEDURE CozeLoopCheckTableExists(
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
DROP PROCEDURE IF EXISTS CozeLoopCheckColumnExists$$
CREATE PROCEDURE CozeLoopCheckColumnExists(
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

-- Function to check if index exists
DELIMITER $$
DROP PROCEDURE IF EXISTS CozeLoopCheckIndexExists$$
CREATE PROCEDURE CozeLoopCheckIndexExists(
    IN p_table_name VARCHAR(64),
    IN p_index_name VARCHAR(64),
    OUT index_exists BOOLEAN
)
BEGIN
    DECLARE index_count INT DEFAULT 0;

    SELECT COUNT(*) INTO index_count
    FROM information_schema.statistics
    WHERE table_schema = DATABASE()
    AND table_name = p_table_name
    AND index_name = p_index_name;

    SET index_exists = (index_count > 0);
END$$
DELIMITER ;

-- Enhanced Universal ALTER TABLE DDL parser and executor
DELIMITER $$
DROP PROCEDURE IF EXISTS CozeLoopExecuteAlterDDL$$
CREATE PROCEDURE CozeLoopExecuteAlterDDL(
    IN p_alter_ddl TEXT
)
BEGIN
    DECLARE table_name VARCHAR(64) DEFAULT '';
    DECLARE operation_type VARCHAR(32) DEFAULT '';
    DECLARE column_name VARCHAR(64) DEFAULT '';
    DECLARE index_name VARCHAR(64) DEFAULT '';
    DECLARE table_exists BOOLEAN DEFAULT FALSE;
    DECLARE column_exists BOOLEAN DEFAULT FALSE;
    DECLARE index_exists BOOLEAN DEFAULT FALSE;
    DECLARE should_execute BOOLEAN DEFAULT TRUE;
    DECLARE sql_stmt TEXT;
    DECLARE error_msg TEXT DEFAULT '';

    SET @alter_sql = UPPER(TRIM(p_alter_ddl));
    -- Clean up any remaining newlines or carriage returns
    SET @alter_sql = REPLACE(@alter_sql, '\n', ' ');
    SET @alter_sql = REPLACE(@alter_sql, '\r', ' ');
    SET @alter_sql = TRIM(@alter_sql);

    SET @table_start = LOCATE('ALTER TABLE', @alter_sql);
    SET @table_end = LOCATE(' ', @alter_sql, @table_start + 12);

    IF @table_end > 0 THEN
        SET table_name = TRIM(SUBSTRING(@alter_sql, @table_start + 12, @table_end - @table_start - 12));
        -- Remove backticks if present
        SET table_name = TRIM(BOTH '`' FROM table_name);
    END IF;

    -- Check if table exists
    IF table_name != '' THEN
        CALL CozeLoopCheckTableExists(table_name, table_exists);
        IF NOT table_exists THEN
            SET error_msg = CONCAT('WARNING: Table ', table_name, ' does not exist, skipping ALTER statement');
            SELECT error_msg as result;
            SET should_execute = FALSE;
        END IF;
    END IF;

    -- Parse operation type and extract relevant information
    IF should_execute THEN
        -- Check for ADD COLUMN
        IF LOCATE('ADD COLUMN', @alter_sql) > 0 THEN
            SET operation_type = 'ADD_COLUMN';
            -- Extract column name - handle both backtick and non-backtick cases
            SET @add_start = LOCATE('ADD COLUMN', @alter_sql);
            SET @col_start = LOCATE('`', @alter_sql, @add_start);
            IF @col_start > 0 THEN
                SET @col_end = LOCATE('`', @alter_sql, @col_start + 1);
                IF @col_end > 0 THEN
                    SET column_name = SUBSTRING(@alter_sql, @col_start + 1, @col_end - @col_start - 1);
                END IF;
            ELSE
                -- Try to extract column name without backticks
                SET @space_after_add = LOCATE(' ', @alter_sql, @add_start + 10);
                IF @space_after_add > 0 THEN
                    SET @next_space = LOCATE(' ', @alter_sql, @space_after_add + 1);
                    IF @next_space > 0 THEN
                        SET column_name = SUBSTRING(@alter_sql, @space_after_add + 1, @next_space - @space_after_add - 1);
                    END IF;
                END IF;
            END IF;

            -- Check if column already exists
            IF column_name != '' THEN
                CALL CozeLoopCheckColumnExists(table_name, column_name, column_exists);
                IF column_exists THEN
                    SET error_msg = CONCAT('Column ', column_name, ' already exists in table ', table_name, ', skipping');
                    SELECT error_msg as result;
                    SET should_execute = FALSE;
                END IF;
            END IF;
        -- Check for ADD INDEX/KEY
        ELSEIF LOCATE('ADD INDEX', @alter_sql) > 0 OR LOCATE('ADD KEY', @alter_sql) > 0 THEN
            SET operation_type = 'ADD_INDEX';
            -- Extract index name
            SET @add_start = GREATEST(
                IFNULL(NULLIF(LOCATE('ADD INDEX', @alter_sql), 0), 0),
                IFNULL(NULLIF(LOCATE('ADD KEY', @alter_sql), 0), 0)
            );
            SET @idx_start = LOCATE('`', @alter_sql, @add_start);
            IF @idx_start > 0 THEN
                SET @idx_end = LOCATE('`', @alter_sql, @idx_start + 1);
                IF @idx_end > 0 THEN
                    SET index_name = SUBSTRING(@alter_sql, @idx_start + 1, @idx_end - @idx_start - 1);
                    -- Check if index already exists
                    CALL CozeLoopCheckIndexExists(table_name, index_name, index_exists);
                    IF index_exists THEN
                        SET error_msg = CONCAT('Index ', index_name, ' already exists in table ', table_name, ', skipping');
                        SELECT error_msg as result;
                        SET should_execute = FALSE;
                    END IF;
                END IF;
            END IF;
        -- Check for ADD UNIQUE INDEX/KEY
        ELSEIF LOCATE('ADD UNIQUE INDEX', @alter_sql) > 0 OR LOCATE('ADD UNIQUE KEY', @alter_sql) > 0 THEN
            SET operation_type = 'ADD_UNIQUE_INDEX';
            -- Extract index name
            SET @add_start = GREATEST(
                IFNULL(NULLIF(LOCATE('ADD UNIQUE INDEX', @alter_sql), 0), 0),
                IFNULL(NULLIF(LOCATE('ADD UNIQUE KEY', @alter_sql), 0), 0)
            );
            SET @idx_start = LOCATE('`', @alter_sql, @add_start);
            IF @idx_start > 0 THEN
                SET @idx_end = LOCATE('`', @alter_sql, @idx_start + 1);
                IF @idx_end > 0 THEN
                    SET index_name = SUBSTRING(@alter_sql, @idx_start + 1, @idx_end - @idx_start - 1);
                    -- Check if index already exists
                    CALL CozeLoopCheckIndexExists(table_name, index_name, index_exists);
                    IF index_exists THEN
                        SET error_msg = CONCAT('Unique index ', index_name, ' already exists in table ', table_name, ', skipping');
                        SELECT error_msg as result;
                        SET should_execute = FALSE;
                    END IF;
                END IF;
            END IF;

        ELSE
            SET operation_type = 'UNKNOWN';
            SELECT CONCAT('Unknown ALTER operation type, executing as-is: ', LEFT(p_alter_ddl, 100)) as result;
        END IF;
    END IF;

    -- Execute the ALTER statement if conditions are met
    IF should_execute THEN
        BEGIN
            DECLARE EXIT HANDLER FOR SQLEXCEPTION
            BEGIN
                SELECT CONCAT('ERROR executing ALTER statement: ', p_alter_ddl) as error;
                RESIGNAL;
            END;

            SET sql_stmt = p_alter_ddl;
            SET @sql = sql_stmt;
            PREPARE stmt FROM @sql;
            EXECUTE stmt;
            DEALLOCATE PREPARE stmt;

            -- Log success
            CASE operation_type
                WHEN 'ADD_COLUMN' THEN
                    SELECT CONCAT('Column ', column_name, ' added to table ', table_name) as result;
                WHEN 'ADD_INDEX' THEN
                    SELECT CONCAT('Index ', index_name, ' added to table ', table_name) as result;
                WHEN 'ADD_UNIQUE_INDEX' THEN
                    SELECT CONCAT('Unique index ', index_name, ' added to table ', table_name) as result;
                ELSE
                    SELECT CONCAT('ALTER statement executed successfully on table ', table_name) as result;
            END CASE;
        END;
    END IF;
END$$
DELIMITER ;

-- Function to execute multiple ALTER statements from a file
DELIMITER $$
DROP PROCEDURE IF EXISTS CozeLoopExecuteAlterFile$$
CREATE PROCEDURE CozeLoopExecuteAlterFile(
    IN p_file_content TEXT
)
BEGIN
    DECLARE done INT DEFAULT FALSE;
    DECLARE current_statement TEXT DEFAULT '';
    DECLARE statement_start INT DEFAULT 1;
    DECLARE statement_end INT DEFAULT 1;
    DECLARE semicolon_pos INT DEFAULT 1;
    DECLARE statement_count INT DEFAULT 0;
    DECLARE success_count INT DEFAULT 0;
    DECLARE skip_count INT DEFAULT 0;

    -- First, clean up the entire file content to normalize whitespace and newlines
    SET p_file_content = REPLACE(p_file_content, '\n', ' ');
    SET p_file_content = REPLACE(p_file_content, '\r', ' ');
    SET p_file_content = REGEXP_REPLACE(p_file_content, '[[:space:]]+', ' ');
    SET p_file_content = TRIM(p_file_content);

    -- Check for content truncation
    IF LENGTH(p_file_content) < 100 THEN
        SELECT CONCAT('WARNING: File content seems too short (', LENGTH(p_file_content), ' chars), may be truncated') as warning;
    END IF;

    WHILE statement_start <= LENGTH(p_file_content) DO
        -- Find next semicolon
        SET semicolon_pos = LOCATE(';', p_file_content, statement_start);

        IF semicolon_pos > 0 THEN
            -- Extract statement
            SET current_statement = SUBSTRING(p_file_content, statement_start, semicolon_pos - statement_start + 1);

            -- Clean up the individual statement
            SET current_statement = TRIM(current_statement);

            -- Skip empty statements and comments
            IF current_statement != '' AND LEFT(current_statement, 2) != '--' AND LEFT(current_statement, 2) != '/*' THEN
                -- Check if it's an ALTER statement (more robust check)
                IF UPPER(current_statement) LIKE 'ALTER TABLE%' THEN
                    SET statement_count = statement_count + 1;
                    SELECT CONCAT('Executing statement #', statement_count, ': ', LEFT(current_statement, 50), '...') as info;

                    BEGIN
                        DECLARE EXIT HANDLER FOR SQLEXCEPTION
                        BEGIN
                            SELECT CONCAT('ERROR in statement #', statement_count, ': ', LEFT(current_statement, 100)) as error;
                            SELECT CONCAT('Full statement: ', current_statement) as error_full;
                            SET skip_count = skip_count + 1;
                        END;

                        CALL CozeLoopExecuteAlterDDL(current_statement);
                        SET success_count = success_count + 1;
                    END;
                ELSE
                    SELECT CONCAT('Skipping non-ALTER statement: ', LEFT(current_statement, 50), '...') as info;
                    SET skip_count = skip_count + 1;
                END IF;
            END IF;

            SET statement_start = semicolon_pos + 1;
        ELSE
            -- No more semicolons, exit
            SET statement_start = LENGTH(p_file_content) + 1;
        END IF;
    END WHILE;

    SELECT CONCAT('Summary: Processed ', statement_count, ' ALTER statements, ', success_count, ' successful, ', skip_count, ' skipped') as summary;
END$$
DELIMITER ;
