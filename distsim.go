// Este programa requiere 2 parámetros de entrada :
//      - Nombre fichero json de Lefs
//        - Número de ciclo final
//
// Ejemplo : ./censim  testdata/Ejemplo1ParaTests.rdp.subred0.json 5
package main

import (
	"centralsim"
	//"os"
	"comm_vector"	
	//"net"
	"time"
	"fmt"
	"strings"
	"strconv"
	"sort"
)
/*
	Dado una lista de transiciones devuelve un diccionario 
	tal que dic[id_trans] = IP_Maquina 
*/
func CrearDic(L *centralsim.TransitionList,ip string,M *map[centralsim.IndLocalTrans]string)(){
	//M:= make(map[centralsim.IndLocalTrans]string);

	for _,trans := range *L{
		id := trans.IiIndLocal;
		//fmt.Println(id);
		(*M)[id] = ip;
	}

	//return M;
}

/*
	Devuelve true si en L hay una transicion con ID = id
*/
func Esta(L* centralsim.TransitionList,id centralsim.IndLocalTrans)(bool){
	found := false;

	for _,trans := range *L{
		found = id == trans.IiIndLocal;
		if (found){
			break;
		}
	}

	return found;
}

/*
	Devuelve una lista con los ids de las transiciones a los que les enviaras eventos
	Origen S
	Destino L
*/
func CheckObjetivo(L *centralsim.TransitionList,S *centralsim.SimulationEngine)([]centralsim.IndLocalTrans){
	lefs := S.GetLefs();
	var I []centralsim.IndLocalTrans;
	//I := make([]centralsim.IndLocalTrans,1);
	for _,trans := range lefs.IaRed{
		if (trans.Ib_de_salida){
			//fmt.Println("DE SALIDA")
			//Los eventos generados estan en otra subred
			for _,elem := range trans.TransConstPul{
				trans2:= elem[0]; //Transicion objetivo
				if (trans2 < 0 ){
					trans2 = -1 * (int(trans2)+1);
				}
				
				found := Esta(L,centralsim.IndLocalTrans(trans2));
				if (found){
					I = append(I,centralsim.IndLocalTrans(trans2));
				}
			}
		}
	}
	return I;
}

/*
	Dada una lista de transiciones devuelve el
	tiempo conjunto de ejecucion de todas ellas
*/
func TiempoTotal(L centralsim.TransitionList)(int){
	acum:=0;
	for _,trans := range L {
		acum += int(trans.IiDuracionDisparo);
		
	}
	
	return acum;
}


/*
	Elimina de un slice el indice s
*/
func remove(slice []string, s int) []string {
    return append(slice[:s], slice[s+1:]...)
}
/*
	Encuentra el maximo valor en un slice
*/
func max(slice []int)(int){
	aux := slice[0]
	for _,elem := range slice[1:]{
		if (elem > aux){
			aux = elem;
		}
	}
	return aux;
}
/*
	Sincroniza el comienzo de la simulacion
*/
func sincronizar(C * comm_vector.Communicator,IPs []string){
	id := C.Id;
	if id == 0 {
		for _,ip := range IPs[1:]{
			C.Try(ip);
		}
	}else{
		C.Init_Receive();
	}
}
/*
	Se calcula el incremento del lookahead a enviar cuando la subred no tenga el token
	El valor obtenido se tendrá que sumar al tiempo actual
	//Además se crea un diccionario tal que 
	dic[Transition_id] = ip_maquina_encargada
*/
func obtener_Incremento_lookAhead(ms *centralsim.SimulationEngine,C *comm_vector.Communicator,IPs []string)(int,map[centralsim.IndLocalTrans]string,map[string]chan comm_vector.Msg,map[string]chan comm_vector.Msg){
	//IPs = remove(IPs,C.Id);
	//fmt.Println(C.Id,IPs);
	lefs := ms.GetLefs();
	//Enviamos transiciones a todos
	for i,ip := range IPs{
		if i!=C.Id{
			C.Send_List(ip,lefs.IaRed_AUX);
		}
	}
	var received comm_vector.Msg_List;
	mapa := make(map[centralsim.IndLocalTrans]string);
	
	/*
		Recibes la lista de las transiciones de todos los procesos
		Respondes Enviando un Evento
		La cte del evento sera el numero de transiciones a las que tu le enviaras eventos en el funturo
		De forma que si el proceso es respondido con un Evento con una cte > 0 significa que tu 
		subred es un canal de entrada de la subred de dicho proceso

		El tiempo del evento será el tiempo total que toman la ejecucion de tus transiciones
	*/
	var E centralsim.Event;
	chan_map_salida := make(map[string]chan comm_vector.Msg );
	for i:=0 ;i<len(IPs)-1;i++{
		received = C.Receive_List()
		CrearDic(&received.L,received.IP,&mapa);
		
		//fmt.Println(C.Id,mapa)
		lista := CheckObjetivo(&received.L,ms);
		//fmt.Println(C.Id,lista)
		objetivo := len(lista)
		E.SetCte(centralsim.TypeConst(objetivo));
		if objetivo > 0 {
			chan_aux := make(chan comm_vector.Msg,10)
			chan_map_salida[received.IP] = chan_aux;
		}
		TtotalLocal := TiempoTotal(lefs.IaRed_AUX);
		E.SetTiempo(centralsim.TypeClock(TtotalLocal));
		C.Send(received.IP,E);
	}

	var L_suma []int;
	canales_entrada := 0 ;
	var Msg_E comm_vector.Msg;
	chan_map_entrada := make(map[string]chan comm_vector.Msg );
	/*
		Recibes la respuesta de todos los procesos
		Si es un canal de entrada anotas el tiempo de su ejecucion para calcular despues el max
		Ademas aniades un canal para ese proceso en el diccionario de canales
	*/
	for i:=0 ;i<len(IPs)-1;i++{
		Msg_E = C.Receive();
		E := Msg_E.Evento;
		if (int(E.IiCte) > 0 ){
			canales_entrada++;
			chan_aux := make(chan comm_vector.Msg,10)
			chan_map_entrada[Msg_E.IP] = chan_aux;
			L_suma = append(L_suma,int(E.IiTiempo));
		}
	}
	/*
		El incremento del lookahead será el tiempo de todas las transiciones
		+ el maximo tiempo de las subredes de entrada
	*/
	max := max(L_suma);
	var lookahead int;
	if C.Id == 0 {
		lookahead= max + TiempoTotal(lefs.IaRed_AUX);
	}else{
		lookahead = TiempoTotal(lefs.IaRed_AUX)
	}
	//fmt.Println(lookahead)
	return lookahead,mapa,chan_map_entrada,chan_map_salida;
}
//Devuelve el menor valor del slice
func min(slice[]int)(int){
	aux:=slice[0];
	for _,elem := range slice{
		if elem < aux{
			aux = elem;
		}
	}
	return aux;
}


/*
	Calcula el incremento de lookahead cuando la subred tenga alguna transicion
	sensibilizada (haya token),el lookahead se calculara como el sumatorio
	de tiempos de transiciones desde la ultima transicion sensibilizada hasta
	la transicion de salida
*/
func Lookahead_Token(ms *centralsim.SimulationEngine)(int){
	var acum int;
	lefs:=ms.GetLefs();
	lefs.ActualizaSensibilizadas(0);
	if lefs.HaySensibilizadas(){
		TransMap := lefs.IaRed;
		keys := make([]int,len(TransMap));
		for key := range TransMap{
			keys = append(keys,int(key));
		}
		sort.Ints(keys);
		found := false; //Encontrada la transicion sensibilizada
		i:= 0 ;
		for !found{
			found = TransMap[centralsim.IndLocalTrans(keys[i])].IiValorLef <=0;
			if !found{
				i++;
			}
		}	
		//i es el indice de la primera transicion sensibilizada
		//Inicializamos el acumulador de tiempo a 0
		acum = int(TransMap[centralsim.IndLocalTrans(keys[i])].IiDuracionDisparo) ;
		salida := TransMap[centralsim.IndLocalTrans(keys[i])].Ib_de_salida;
		indice:=keys[i];
		for !salida{
			transicion := TransMap[centralsim.IndLocalTrans(indice)]
			salida = transicion.Ib_de_salida;
			
			//Si esta sensibilizada se vuelve a empezar
			if transicion.IiValorLef <=0{
				acum = 0 ;
			}

			//Aumentamos tiempo
			acum += int(transicion.IiDuracionDisparo);

			if !salida{
				trans_obj := transicion.TransConstPul;
				//Esto no deberia pasar en este problema
				if len(trans_obj) > 1{
					fmt.Println("MAS DE UNA TRANSICION OBJETIVO");
				}
				T_siguiente := trans_obj[0];
				indice = T_siguiente[0];

			}

			
		}

	

	}else{
		fmt.Println("NO HAY SENSIBILIZADAS EN LOOKAHEAD TOKEN");
		acum = -1;
	}
	return acum;

}
/*
	Comprueba si hay al menos un mensaje en el buffer de cada 
	canal
*/
func hay_uno(M *map[string]chan comm_vector.Msg)(bool){
	aux:= true;
	for _,channel := range *M{
		if aux{
			aux = len(channel) >0;
		}
	}
	return aux;
}
/*
	Recibe mensaje y lo envia al canal correspondiente
	Termina cuando se notifica
*/
func Recibir_Mensajes(C *comm_vector.Communicator,M *map[string]chan comm_vector.Msg,chan_fin *chan comm_vector.Msg){
	
	
	for len(*chan_fin) == 0 {
		
		received := C.Receive();
		
			
		(*M)[received.IP]<-received
		
	}
}
	
/*
	Recibe los eventos de los canales
*/
func sacarEventos(M *map[string]chan comm_vector.Msg,chan_fin *chan comm_vector.Msg,fin *bool)([]centralsim.Event){
	var aux []centralsim.Event;
	for _,channel := range *M{
		select{
		case msg := <-channel:
			if msg.Evento.IiTransicion <0 {
				msg.Evento.IiTransicion = centralsim.IndLocalTrans(-1 * (int(msg.Evento.IiTransicion) + 1));
			}
			aux = append(aux,msg.Evento);
		case _= <- *chan_fin:
			*fin = true;
		

		}
	}
	*fin = *fin || len(*chan_fin) >0;
	return aux;

}
/*
	Devuelve si una cadena esta en la lista de cadenas
*/
func esta(L []string ,cadena string)(bool){
	encontrado :=false;
	i:=0;
	for !encontrado && i < len(L){
		encontrado = L[i] == cadena;
		i++;
	}
	return encontrado;
}
/*
	Obtiene la transicion de entrada a partir
	de la direccion IP de host
*/
func get_Trans_from_Host(M map[centralsim.IndLocalTrans]string,IP string)(centralsim.IndLocalTrans){
	for ind_trans,IP_salida := range M{
		if IP_salida == IP{
			return ind_trans;
		}
	}
	fmt.Println("ADVERTENCIA EN GET TRANS FROM HOST");
	return -1;
}
/*
	Envia los eventos correspondientes a las subredes de salida
	Los eventos pueden ser null o no
*/
func Enviar_Eventos(ms*centralsim.SimulationEngine,L []centralsim.Event,C *comm_vector.Communicator,M map[centralsim.IndLocalTrans]string,lookahead_no_token int,salidas []string,isEnd bool){
	var enviados []string; //Ip de las subredes a las que se ha enviado un evento
	var EventoAux centralsim.Event;
	//fmt.Println("HOLA SOY",C.Id);
	for _,event := range L{
		
		ip_Destino := M[event.IiTransicion];
		event.IsNull = false;
		event.IsEnd = isEnd;
		C.Send(ip_Destino,event);
		if !esta(enviados,ip_Destino){
			enviados = append(enviados,ip_Destino);
		}
	}
	for _,IP_salida := range salidas{
		//fmt.Println(IP_salida);
		if !esta(enviados,IP_salida){
			//No se ha enviado evento asi que se envia Null
			var lookahead int;
			if ms.HaySensibilizadas(){
				lookahead = Lookahead_Token(ms);
			}else{
				lookahead = lookahead_no_token;
			}
			EventoAux = centralsim.Event{ms.GetLocalTime() + centralsim.TypeClock(lookahead),get_Trans_from_Host(M,IP_salida),0,true,isEnd};
			C.Send(IP_salida,EventoAux);
			enviados = append(enviados,IP_salida)
		}
	}

	//time.Sleep(time.Duration(C.Id) * 600 * time.Millisecond);
	//fmt.Println(C.Id,enviados);

}
/*
	Obtiene el menor lookahead de una lista de Eventos (contando solo los null)
*/
func obtener_min_lookahead(L []centralsim.Event)(centralsim.TypeClock){
	min := centralsim.TypeClock(-1);
	if len(L) > 0  {
		if L[0].IsNull{
			min = L[0].IiTiempo;
		}
		
		i :=1;
		for  i < len(L){
			if L[i].IiTiempo < min && L[i].IsNull{
				min = L[i].IiTiempo;
			}
			i++;
		}
		
	}else{
		fmt.Println("NO EVENTOS EN OBTENER MIN LOOKAHEAD");
		min = -1;
	}
	return min;
}
/*
	Trata los indices negativos de los Eventos
*/
func tratar_eventos(L *[]centralsim.Event){
	for i,_ := range *L{
		if (*L)[i].IiTransicion < 0 {
			(*L)[i].IiTransicion = -1* ((*L)[i].IiTransicion+1);
		}
	}
}
/*
	Devuelve el minimo entre 2 valores de reloj
*/
func min_reloj(a centralsim.TypeClock,b centralsim.TypeClock)(centralsim.TypeClock){
	if a < b{
		return a;
	}else{
		return b;
	}
}

/*
	IPs -> slice con las IP de TODOS los procesos
	filename_json -> Nombre del fichero json a cargar
	id -> id del proceso, corresponde al indice de la IP del proceso en IPs
*/
func Simulador(IPs []string,filename string,id int,CicloFinal int,disparadas *[]centralsim.ResultadoTransition){
	//Creamos el objeto communicator
	C:=comm_vector.Communicator{};
	puerto:=strings.Split(IPs[id],":")[1]
	file_log := "Proceso_"+strconv.Itoa(id);
	C.Init(IPs[id],puerto,file_log,id);

	//Creamos el simulation engine
	lefs, err := centralsim.Load(filename);
	if err != nil {
		println("Couln't load the Petri Net file !")
	}
	ms := centralsim.MakeMotorSimulation(lefs)

	//Se sincroniza el evento de la simulacion
	sincronizar(&C,IPs);


	//Obtenemos los lookahead
	//lookahead[0]->Si la subnet tiene el token
	//lookahead[1]->Si la subnet no tiene el token
	lookahead_no_token,mapa_trans,mapa_chan_entrada,mapa_chan_salida := obtener_Incremento_lookAhead(&ms,&C,IPs);

	var IPs_salida []string;

	for key,_ := range mapa_chan_salida{
		IPs_salida = append(IPs_salida,key);
	}

	// time.Sleep(time.Duration(C.Id) * 200 * time.Millisecond);
	// fmt.Println(C.Id);
	// fmt.Println("=====================")
	// fmt.Println("Mi lookahead cuando no tengo token: ",lookahead_no_token);
	// for trans_id,ip := range mapa_trans{
	// 	fmt.Println("Transicion ",trans_id," en ",ip)
	// }
	// fmt.Println("Canales de salida: ");
	// for key,_ := range mapa_chan_salida{
	// 	fmt.Println(key);
	// }
	// fmt.Println("Canales de entrada: ");
	// for key,_ := range mapa_chan_entrada{
	// 	fmt.Println(key);
	// }
	// if C.Id == 0{
	// 	fmt.Println("Mi lookahead cuando tengo token",Lookahead_Token(&ms));
	// }

	/*
		Actualizar sensibilizadas
		for
			Enviar lookahead
				-Comprobar que lookahead te toca
				-Enviar a todos los canales de salida
			Recibir lookaheads
			Procesar Eventos
			Simular lo correspondiente
		
	*/
	ms.ActualizaSensibilizadas(0);
	fin :=false;
	var recibidos []centralsim.Event;
	
	Enviar_Eventos(&ms,recibidos,&C,mapa_trans,lookahead_no_token,IPs_salida,false);

	ms.VaciaSensibilizada();

	
	chan_fin := make(chan comm_vector.Msg,10)
	go Recibir_Mensajes(&C,&mapa_chan_entrada,&chan_fin);
	acum := 0 ;
	for ms.GetLocalTime() < centralsim.TypeClock(CicloFinal) {
		//fmt.Println("Entro ",C.Id);
		
		
		//time.Sleep(200 * time.Millisecond)
		//fmt.Println("Salgo ",C.Id);
	
		
		Eventos_recibidos := sacarEventos(&mapa_chan_entrada,&chan_fin,&fin);
		
		//LOG
		if false && C.Id != 0{
			time.Sleep(time.Duration(C.Id) * 400 * time.Millisecond);
			fmt.Println(C.Id);
			fmt.Println("=======================")
			for _,evento := range Eventos_recibidos{
				
				if evento.IsNull{
					fmt.Println("Recibido evento NULL T=",evento.IiTiempo);
				}else{
					//fin = true;

					fmt.Printf("NO NULL T =%d Trans = %d Tiempo Local =%d \n",evento.IiTiempo,evento.IiTransicion,int(ms.GetLocalTime()));
				}
			}
		}	
	
		min_ahead := obtener_min_lookahead(Eventos_recibidos);
		max_time := centralsim.TypeClock(-1);
		min_time := centralsim.TypeClock(CicloFinal);
		algun_End := false;
		for _,evento := range Eventos_recibidos{

			if evento.IsEnd{
				algun_End = true;
			}
			if !evento.IsNull{
				//No es null,se aniade
				if evento.IiTiempo > max_time {
					max_time = evento.IiTiempo;
				}
				if evento.IiTiempo < min_time {
					min_time = evento.IiTiempo;
				}
				//fmt.Println("Aniado ",C.Id);
				if evento.IiTiempo < centralsim.TypeClock(CicloFinal){
					
					ms.AniadeEvento(evento);
				}
				
			}

		}
		
		//Simulacion local
		if algun_End{
			//Se simula desde el tiempo local hasta el final
			ms.SimularPeriodo(ms.GetLocalTime(),centralsim.TypeClock(CicloFinal));
		}else{
			if int(min_ahead) == -1{
					acum = 0 ;
				//No hay evento null
				//if C.Id == 0 {
					//fmt.Println("PRE",C.Id,ms.GetLocalTime());
					//Se adelanta el tiempo para que se pueda lanzar la transicion
					ms.SimularPeriodo(ms.GetLocalTime(),min_time);
					ms.TratarEventos();
					//fmt.Println("POST",C.Id,ms.GetLocalTime());
					//fmt.Println(ms.ComprobarSensibilizadas())
					ms.SimularUnpaso(min_reloj(max_time,centralsim.TypeClock(CicloFinal)));
					//fmt.Println(ms.ComprobarSensibilizadas())
				//}
			}else{
				//fmt.Println("BBBBBBBBBBBBBBBBBBBBBBBB")
				//Hay que avanzar hasta el min lookahead
				//Si no hay eventos simplemente no hara nada, solo avanzar el reloj
				
				if ms.ComprobarSensibilizadas(){
					// if C.Id ==0 {
					// 	fmt.Println(min_ahead);
					// }
					ms.SimularPeriodo(ms.GetLocalTime(),min_reloj(min_ahead,centralsim.TypeClock(CicloFinal)));
				}else{
					if C.Id != 0 {
						acum++;
					}
				}
			}
		}
		
		//LOG
		
		lista_aux := []centralsim.Event(ms.ListaEventosFuera);
		tratar_eventos(&lista_aux);
		//time.Sleep(time.Duration(C.Id) * 600 * time.Millisecond);
		// if C.Id == 0 {
		// 	fmt.Println("ID = ",C.Id);
		// 	fmt.Println("EVENTOS GENERADOS ",len(ms.ListaEventosFuera));
		// 	fmt.Println("=================")
		// }
		// for _,evento := range lista_aux{
		// 	fmt.Println("Evento Trans = ",evento.IiTransicion);
		// }
		
		//Enviar eventos afuera
		if int(ms.GetLocalTime()) >= CicloFinal{
			//chan_fin <-comm_vector.Msg{};
			fin = true;
			Enviar_Eventos(&ms,lista_aux,&C,mapa_trans,lookahead_no_token,IPs_salida,true);
			C.Event("Decido finalizar");
			
			
		}else{
			if C.Id!= 0 {
				//fmt.Println("Hola")
				Enviar_Eventos(&ms,lista_aux,&C,mapa_trans,lookahead_no_token+acum,IPs_salida,false);
			}else{
				Enviar_Eventos(&ms,lista_aux,&C,mapa_trans,lookahead_no_token,IPs_salida,false);
			}
			
			var aux centralsim.EventList;
			ms.ListaEventosFuera = aux; //vaciamos
			//fin = true;
		}
		//fin = len(chan_fin) > 0 ;
		
	}
	*disparadas = ms.IvTransResults;
	C.Event("FINALIZO")
	//fmt.Println("He terminado ",C.Id)

	//fmt.Printf(filename,lookahead);
	//fmt.Printf(filename,mapa_trans);
	//fmt.Printf(filename,mapa_chan);
}

func Init(IPs []string,filename string,id int,CicloFinal int){
	var procesados []centralsim.ResultadoTransition;
	Simulador(IPs,filename,id,CicloFinal,&procesados);
	fmt.Println(id,procesados)
	
}
func main() {
	// cargamos un fichero de estructura Lef en formato json para centralizado
	// os.Args[0] es el nombre del programa que no nos interesa
	IPs := []string{"localhost:30000","localhost:40000","localhost:50000"};
	subredes := "./testdata/red_2_2_3.rdp.subred";
	CicloFinal := 12;
	for i,_ := range IPs{
		filename := subredes+strconv.Itoa(i)+".json";
		if i != len(IPs)-1 {
			go Init(IPs,filename,i,CicloFinal);
		}else{
			Init(IPs,filename,i,CicloFinal)
		}
	}
	time.Sleep(1000 * time.Millisecond);
}
