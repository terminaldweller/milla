CREATE OR REPLACE FUNCTION milla.janitor ()
    RETURNS TRIGGER
    AS $$
BEGIN
    UPDATE
        posts
    SET
        updated_at = now()
    WHERE
        id = NEW.id;
    RETURN new;
END;
$$
LANGUAGE plpgsql;

CREATE TRIGGER janitor_trigger
    AFTER INSERT ON milla.tables
    EXECUTE PROCEDURE milla.janitor ();

