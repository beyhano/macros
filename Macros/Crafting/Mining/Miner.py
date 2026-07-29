# Miner.py

# Name: Mining Random Macro
# Description: MACRO MINER RANDOM - TROY UO
# Shard: Troy UO
# Date: 2026-07-19

import System
from System import Random

print("=== TROY UO - MINER RANDOM ===")
print("Once kazmani hedef goster!")
PromptAlias('pickaxe')
pickaxeGraphic = Graphic('pickaxe')
print("Simdi pack llama'yi hedef goster!")
PromptAlias('Packlhama')

def get_new_pickaxe():
    if FindType(pickaxeGraphic, -1, 'backpack'):
        SetAlias('pickaxe', GetAlias('found'))
        HeadMsg("Yeni kazma!")
        return True
    return False

def guardar():
    HeadMsg("Guardando...")
    llama = GetAlias('Packlhama')
    if llama == '' or llama == 0:
        return
    UseObject(llama)
    Pause(800)
    for oid in [0x19B7, 0x19B8, 0x19B9, 0x19BA, 0x19BB, 0x19BC, 0x19BD, 0x19BE, 0x19BF]:
        while FindType(oid, -1, 'backpack'):
            MoveItem(GetAlias('found'), llama)
            Pause(500)

yonsira = 0
yonler = ['North','Northeast','East','Southeast','South','Southwest','West','Northwest']

def Mine():
    global yonsira
    HeadMsg("Minerando...")
    last_w = Weight()
    stale = 0
    while stale < 5:
        if InJournal('You have worn out your tool!'):
            ClearJournal()
            get_new_pickaxe()
            Pause(500)
        # Kazmayi al
        p = GetAlias('pickaxe')
        if p == '' or p == 0:
            get_new_pickaxe()
            Pause(500)
            p = GetAlias('pickaxe')
        if p == '' or p == 0:
            stale += 1
            continue
        # Siradaki yone don ve dene
        Turn(yonler[yonsira])
        yonsira = (yonsira + 1) % 8
        Pause(300)
        ClearJournal()
        CancelTarget()
        Pause(200)
        UseObject(p)
        if WaitForTarget(2000):
            TargetTileRelative("self", 1)
            Pause(1200)
            if Weight() > last_w:
                last_w = Weight()
                stale = 0
            else:
                stale += 1
        else:
            stale += 1
        if Weight() >= 300:
            guardar()
    HeadMsg("Proximo local...")

# Ana dongu
dirOff = {'East':(1,0),'West':(-1,0),'North':(0,-1),'South':(0,1),
          'Northeast':(1,-1),'Southeast':(1,1),'Southwest':(-1,1),'Northwest':(-1,-1)}
dirList = ['East','West','North','South','Northeast','Southeast','Southwest','Northwest']
rnd = Random()

while True:
    Mine()
    Pause(500)
    # Rastgele 1-3 adim yuru
    for _ in range(rnd.Next(3) + 1):
        d = dirList[rnd.Next(8)]
        dx, dy = dirOff[d]
        Pathfind(X("self") + dx, Y("self") + dy, Z("self"))
        Pause(1500)
    Pause(500)
