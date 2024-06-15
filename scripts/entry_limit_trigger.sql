create function remove_old_entries()
    returns trigger
    as $$
begin
    if(
        select
            COUNT(*)
        from
            table_name) > 10000 then
        delete from table_name
        where id in(
                select
                    id
                from
                    table_name
                order by
                    id asc
                limit 1000);
    end if;
    return null;
end;
$$
language plpgsql;

create trigger remove_old_entries_trigger
    after insert on table_name for each row
    execute procedure remove_old_entries();
