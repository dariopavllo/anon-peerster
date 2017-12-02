@echo off
del gossiper.exe
go build
mv Homework3.exe gossiper.exe

start "NodeA" cmd /K gossiper -name=NodeA -gossipAddr=127.0.0.1:5001 -peers=127.0.0.1:5002 -UIPort=8080
start "NodeB" cmd /K gossiper -name=NodeB -gossipAddr=127.0.0.1:5002 -peers=127.0.0.1:5001,127.0.0.1:5003
start "NodeC" cmd /K gossiper -name=NodeC -gossipAddr=127.0.0.1:5003 -peers=127.0.0.1:5002 -UIPort=8081