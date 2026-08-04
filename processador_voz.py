import speech_recognition as sr
import socket
from playsound import playsound
import pyautogui
import threading
import json
import time

pyautogui.PAUSE = 0.01
# Configurações do seu ecossistema Jarvis/Athena
WAKE_WORDS = ["athena", "atena", "Athena", "athenas", "atenas"]  
STOP_WORD = "enviar"
GO_HOST = "127.0.0.1"
GO_PORT = 8081

def enviar_para_go(texto, sock):
    try:
        sock.sendall(f"{texto}\n".encode('utf-8'))
        print(f"  -> 🚀 Comando enviado: {texto}")
        sock.close()
    except Exception as e:
        print(f"  -> ❌ Erro ao conectar no backend Go: {e}")

def falar_comando(recognizer, source, sock):
    buffer_texto = []
    print(">>> Gravando... (Diga 'ENVIAR' para finalizar)")
    
    while True:
        try:
            audio = recognizer.listen(source, timeout=5, phrase_time_limit=10)
            parcial = recognizer.recognize_google(audio, language="pt-BR").lower()
            
            if STOP_WORD in parcial:
                limpo = parcial.replace(STOP_WORD, "").strip()
                if limpo: buffer_texto.append(limpo)
                
                texto_final = " ".join(buffer_texto).strip()
                 # Se você tiver um arquivo .wav legal
                playsound("audios/faaah.wav")
                
                if texto_final:
                    enviar_para_go(texto_final, sock)
                break
            else:
                buffer_texto.append(parcial)
                print(f"  [Acumulando] {parcial}")
                
        except (sr.WaitTimeoutError, sr.UnknownValueError):
            print("... aguardando finalização ou continuação ...")
            continue
        except Exception as e:
            print(f"Erro no comando: {e}")
            break

def ouvir_continuamente(sock):
    recognizer = sr.Recognizer()
    recognizer.dynamic_energy_threshold = True 

    with sr.Microphone() as source:
        print("--- Sincronizando Athena com o ambiente ---")
        recognizer.adjust_for_ambient_noise(source, duration=1)
        print(">>> Escuta ativa iniciada.")

        while True:
            try:
                audio = recognizer.listen(source, phrase_time_limit=3)
                texto = recognizer.recognize_google(audio, language="pt-BR").lower()
                
                print(f"Escutado: {texto}")

                if any(wake_word in texto for wake_word in WAKE_WORDS):
                    print("\n[WAKE] Athena ativada!")
                    playsound("audios/entrada.mp3")
                    
                    falar_comando(recognizer, source, sock)
                    print("\n>>> Voltando para escuta passiva...")

            except (sr.UnknownValueError, sr.WaitTimeoutError):
                continue
            except sr.RequestError:
                print(" Erro de conexão com o Google. Verifique sua internet.")
            except KeyboardInterrupt:
                break

def manipular_tela(sock):
    try:
        while True:
            data = sock.recv(1024).decode('utf-8').strip()
            if not data:
                break
            comando = json.loads(data)
            acao = comando.get("acao")
            
            if acao == "click":
                x = int(comando.get("x", 0))
                y = int(comando.get("y", 0))
                print(f"[Python] Executando Clique em X:{x} Y:{y}")
                pyautogui.moveTo(x, y, duration=1.0)
                pyautogui.click()
                
            elif acao == "tecla":
                texto = comando.get("texto", "")
                print(f"[Python] Pressionando Tecla: {texto}")
                pyautogui.press(texto)
                
    except Exception as e:
        print(f"Erro ao manipular tela: {e}")

def iniciar_conexao():
    while True:
        try:
            sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            sock.settimeout(3) 
            
            print(f"[Python] Tentando conectar ao Core em Go ({GO_HOST}:{GO_PORT})...")
            sock.connect((GO_HOST, GO_PORT))            
            sock.settimeout(None) 
            print("[Python] Conectado ao Core em Go com sucesso!")
            thread_ouvinte = threading.Thread(target=manipular_tela, args=(sock,), daemon=True)
            thread_ouvinte.start()
            
            return sock
            
        except (socket.error, ConnectionRefusedError):
            print("[Python] Servidor Go ainda não iniciou. Tentando novamente em 2 segundos...")
            time.sleep(2)

if __name__ == "__main__":    
    ouvir_continuamente(iniciar_conexao())