Things noticed while using OniWorks that were fixed — route param syntax, ORM map insert, and Raw query discoverability.

Rough edges I noticed:
  - {filename} in routes looks like it should work but it's actually invalid —      
  :filename is the correct syntax. Easy to trip on if you're used to Laravel/Express   style braces
  - Insert() on the ORM panics on map — only accepts structs. Had to build a custom 
  InsertMap() helper with raw SQL to work around it
  - The Raw().All() for SELECT queries — wasn't obvious if it was supported, ended  
  up doing Go-side grouping to be safe