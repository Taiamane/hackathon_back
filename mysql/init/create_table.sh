#!/bin/sh

CMD_MYSQL="mysql -u${MYSQL_USER} -p${MYSQL_PASSWORD} ${MYSQL_DATABASE}"
$CMD_MYSQL -e "create table user (
    id char(26)  AUTO_INCREMENT NOT NULL primary key,
    name varchar(50) NOT NULL,
    age int
    );"
$CMD_MYSQL -e  "insert into user values (00000000000000000000000001, 'hanako', '20');"
$CMD_MYSQL -e  "insert into user values (00000000000000000000000002, 'taro','30');"