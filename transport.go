package main

const CMD_RESPONSE = 128;
const CMD_GETTER = 64;

const CMD_RESET = 0;
const CMD_POS = 1;
const CMD_PID = 2;

const CMD_GETPOS = CMD_GETTER | CMD_POS;
const CMD_GETPID = CMD_GETTER | CMD_PID;
const CMD_GETDIM = CMD_GETTER | (CMD_PID + 1);

const CMD_MEASUREMENT = 0 | CMD_RESPONSE;
const CMD_ERROR_RESPONSE = 255;

type Measurement struct {
	CX, CY float32
	VX, VY float32
	POSX, POSY float32
	RVX, RVY float32
	RAX, RAY float32
	NX, NY float32
	RX, RY float32
	USX, USY float32
	RAWX, RAWY float32
}
