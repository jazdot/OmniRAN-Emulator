package handler

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"OmniRAN-Emulator/internal/control_test_engine/ue/context"
	"OmniRAN-Emulator/internal/control_test_engine/ue/nas/message/nas_control"
	"OmniRAN-Emulator/internal/control_test_engine/ue/nas/message/nas_control/mm_5gs"
	"OmniRAN-Emulator/internal/control_test_engine/ue/nas/message/sender"
	"OmniRAN-Emulator/lib/nas"
	"OmniRAN-Emulator/lib/nas/nasMessage"
	"time"
)

func HandlerAuthenticationReject(ue *context.UEContext, message *nas.Message) {

	log.Info("[UE][NAS] Authentication of UE ", ue.GetUeId(), " failed")

	ue.SetStateMM_DEREGISTERED()
}

func HandlerAuthenticationRequest(ue *context.UEContext, message *nas.Message) {
	var authenticationResponse []byte

	// getting RAND and AUTN from the message.
	rand := message.AuthenticationRequest.GetRANDValue()
	autn := message.AuthenticationRequest.GetAUTN()

	// getting resStar
	paramAutn, check := ue.DeriveRESstarAndSetKey(ue.UeSecurity.AuthenticationSubs, rand[:], ue.UeSecurity.Snn, autn[:])

	switch check {

	case "MAC failure":
		log.Info("[UE][NAS][MAC] Authenticity of the authentication request message: FAILED")
		log.Info("[UE][NAS] Send authentication failure with MAC failure")
		authenticationResponse = mm_5gs.AuthenticationFailure("MAC failure", "", paramAutn)
		// not change the state of UE.

	case "SQN failure":
		log.Info("[UE][NAS][MAC] Authenticity of the authentication request message: OK")
		log.Info("[UE][NAS][SQN] SQN of the authentication request message: INVALID")
		log.Info("[UE][NAS] Send authentication failure with Synch failure")
		authenticationResponse = mm_5gs.AuthenticationFailure("SQN failure", "", paramAutn)
		// not change the state of UE.

	case "successful":
		// getting NAS Authentication Response.
		log.Info("[UE][NAS][MAC] Authenticity of the authentication request message: OK")
		log.Info("[UE][NAS][SQN] SQN of the authentication request message: VALID")
		log.Info("[UE][NAS] Send authentication response")
		authenticationResponse = mm_5gs.AuthenticationResponse(paramAutn, "")

		// change state of UE for registered-initiated
		ue.SetStateMM_REGISTERED_INITIATED()
	}

	// sending to GNB
	sender.SendToGnb(ue, authenticationResponse)
}

func HandlerSecurityModeCommand(ue *context.UEContext, message *nas.Message) {

	switch message.SecurityModeCommand.SelectedNASSecurityAlgorithms.GetTypeOfCipheringAlgorithm() {
	case 0:
		log.Info("[UE][NAS] Type of ciphering algorithm is 5G-EA0")
	case 1:
		log.Info("[UE][NAS] Type of ciphering algorithm is 128-5G-EA1")
	case 2:
		log.Info("[UE][NAS] Type of ciphering algorithm is 128-5G-EA2")
	}

	switch message.SecurityModeCommand.SelectedNASSecurityAlgorithms.GetTypeOfIntegrityProtectionAlgorithm() {
	case 0:
		log.Info("[UE][NAS] Type of integrity protection algorithm is 5G-IA0")
	case 1:
		log.Info("[UE][NAS] Type of integrity protection algorithm is 128-5G-IA1")
	case 2:
		log.Info("[UE][NAS] Type of integrity protection algorithm is 128-5G-IA2")
	}

	// checking BIT RINMR that triggered registration request in security mode complete.
	rinmr := message.SecurityModeCommand.Additional5GSecurityInformation.GetRINMR()

	// getting NAS Security Mode Complete.
	securityModeComplete, err := mm_5gs.SecurityModeComplete(ue, rinmr)
	if err != nil {
		log.Fatal("[UE][NAS] Error sending Security Mode Complete: ", err)
	}

	// sending to GNB
	sender.SendToGnb(ue, securityModeComplete)
}

func HandlerRegistrationAccept(ue *context.UEContext, message *nas.Message) {

	// change the state of ue for registered
	ue.SetStateMM_REGISTERED()

	// saved 5g GUTI and others information.
	ue.SetAmfRegionId(message.RegistrationAccept.GetAMFRegionID())
	ue.SetAmfPointer(message.RegistrationAccept.GetAMFPointer())
	ue.SetAmfSetId(message.RegistrationAccept.GetAMFSetID())
	ue.Set5gGuti(message.RegistrationAccept.GetTMSI5G())

	// use the slice allowed by the network
	// in PDU session request
	if ue.PduSession.Snssai.Sst == 0 {

		// check the allowed NSSAI received from the 5GC
		snssai := message.RegistrationAccept.AllowedNSSAI.GetSNSSAIValue()

		// update UE slice selected for PDU Session
		ue.PduSession.Snssai.Sst = int32(snssai[1])
		ue.PduSession.Snssai.Sd = fmt.Sprintf("0%x0%x0%x", snssai[2], snssai[3], snssai[4])

		log.Warn("[UE][NAS] ALLOWED NSSAI: SST: ", ue.PduSession.Snssai.Sst, " SD: ", ue.PduSession.Snssai.Sd)
	}

	log.Info("[UE][NAS] UE 5G GUTI: ", ue.Get5gGuti())

	// getting NAS registration complete.
	registrationComplete, err := mm_5gs.RegistrationComplete(ue)
	if err != nil {
		log.Fatal("[UE][NAS] Error sending Registration Complete: ", err)
	}

	// sending to GNB
	sender.SendToGnb(ue, registrationComplete)

	// waiting receive Configuration Update Command.
	time.Sleep(20 * time.Millisecond)

	// getting ul nas transport and pduSession establishment request.
	defaultPduSessionId := ue.GetPduSesssionId()
	ulNasTransport, err := mm_5gs.UlNasTransport(ue, defaultPduSessionId, nasMessage.ULNASTransportRequestTypeInitialRequest)
	if err != nil {
		log.Fatal("[UE][NAS] Error sending ul nas transport and pdu session establishment request: ", err)
	}

	// change the state of this session to pending.
	ue.GetPduSession(defaultPduSessionId).State = context.SM5G_PDU_SESSION_ACTIVE_PENDING

	// sending to GNB
	sender.SendToGnb(ue, ulNasTransport)
}

func HandlerDlNasTransportPduaccept(ue *context.UEContext, message *nas.Message) {

	//getting PDU Session establishment accept.
	payloadContainer := nas_control.GetNasPduFromPduAccept(message)
	if payloadContainer == nil {
		log.Error("[UE][NAS] Error: payloadContainer is nil in PDU Accept")
		return
	}
	if payloadContainer.GsmHeader.GetMessageType() == nas.MsgTypePDUSessionEstablishmentAccept {
		log.Info("[UE][NAS] Receiving PDU Session Establishment Accept")

		// get PDU Session ID from payloadContainer
		pduSessionId := payloadContainer.PDUSessionEstablishmentAccept.PDUSessionID.GetPDUSessionID()

		// update PDU Session state to active.
		ue.GetPduSession(pduSessionId).State = context.SM5G_PDU_SESSION_ACTIVE

		// get UE ip
		UeIp := payloadContainer.PDUSessionEstablishmentAccept.GetPDUAddressInformation()
		ue.SetIp(pduSessionId, UeIp)
	}
}

func HandlerDeregistrationRequestUETerminatedDeregistration(ue *context.UEContext, message *nas.Message) {
	log.Info("[UE][NAS] Handling network-initiated Deregistration Request")

	// 1. Terminate/cleanup all virtual interfaces, routes, rules, etc.
	ue.Terminate()

	// 2. Generate Deregistration Accept response
	deregAccept, err := mm_5gs.DeregistrationAcceptUETerminated(ue)
	if err != nil {
		log.Error("[UE][NAS] Error generating Deregistration Accept: ", err)
		return
	}

	// 3. Send the accept response to GNodeB
	sender.SendToGnb(ue, deregAccept)

	// 4. Update the GMM/SM state
	ue.SetStateMM_DEREGISTERED()
	ue.SetStateSM_PDU_SESSION_INACTIVE()

	log.Warn("[UE][NAS] Deregistration completed. UE state set to DEREGISTERED")
}
