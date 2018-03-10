let timer = null
let tabCounter = 2

$(document).ready(function(){
	update()
	timer = setInterval(update, 1000)
	
	$.fn.exists = function() {
		return this.length !== 0
	}
	
	$("#sendMessage").click(function() {
		const msg = $("#message").val()
		
		$("#sendMessage").prop("disabled", true)
		$("#message").prop("disabled", true)
		$("#loading").show()
		
		// Check selected tab
		const nodeName = $('li[aria-selected="true"]').attr("data-nodename")
		if (typeof nodeName !== typeof undefined && nodeName !== false) {
			// Private message
			$.ajax({
				type: 'POST',
				url: "/privateMessage",
				data: JSON.stringify({Destination: nodeName, Content: msg}),
				success: function() {
					update()
					$("#sendMessage").prop("disabled", false)
					$("#message").prop("disabled", false)
					$("#message").val("")
					$("#loading").hide()
				},
				error: function() {
					alert("Unable to send private message")
					$("#sendMessage").prop("disabled", false)
					$("#message").prop("disabled", false)
					$("#loading").hide()
				},
				contentType: "application/json"
			})
		} else {
			// Generic gossip message
			$.ajax({
				type: 'POST',
				url: "/message",
				data: JSON.stringify(msg),
				success: function() {
					update()
					$("#sendMessage").prop("disabled", false)
					$("#message").prop("disabled", false)
					$("#message").val("")
					$("#loading").hide()
					$('#tabs-1').scrollTop(1E10);
				},
				error: function() {
					alert("Unable to send gossip message")
					$("#sendMessage").prop("disabled", false)
					$("#message").prop("disabled", false)
					$("#loading").hide()
				},
				contentType: "application/json"
			})
		}
	})
	
	$("#addPeer").click(function(){
		const peer = $("#newPeerAddress").val()
		$.ajax({
			type: 'POST',
			url: "/node",
			data: JSON.stringify(peer),
			success: function() {
				update()
			},
			error: function() {
				alert("Unable to add peer")
			},
			contentType: "application/json"
		})
    })
	
	$("#tabs").tabs()
})

function showMessages(container, messages, myName) {
	if (container !== null) {
		container.innerHTML = ""
		messages.forEach(m => {
			const elem = document.createElement("div")
			if (m.FromNode == myName) {
				elem.className = "myName"
			}
			const tooltip = document.createElement("img")
			tooltip.src = "info.png"
			tooltip.title = "Message first seen on " + m.FirstSeen + " \n"
				+ "Relayed through " + m.FromAddress + " \n"
				+ "Sequence ID: " + m.SeqID + " \n"
				+ "Hash: " + m.Hash
			elem.appendChild(tooltip)
			const nameTag = document.createElement("span")
			const date = m.FirstSeen.slice(0, 10)
			nameTag.appendChild(document.createTextNode(" " + m.FromNode + " "))
			nameTag.title = tooltip.title
			elem.appendChild(nameTag)
			if (m.SeqID == 0) {
				const bold = document.createElement("strong")
				bold.appendChild(document.createTextNode(m.Content))
				elem.appendChild(bold)
			} else {
				elem.appendChild(document.createTextNode(m.Content))
			}
			container.appendChild(elem)
		})
	}	
}

function update() {
	$.when(
		$.get("/id"),
		$.get("/node"),
		$.get("/message"),
		$.get("/routes")
	)
	.then(function(id, nodes, messages, routes) {
		const name = JSON.parse(id[0])
		$(".nodeName").text(name)
		
		showMessages(document.getElementById("tabs-1"), JSON.parse(messages[0]), name)
		
		const peerBox = document.getElementById("peerContent")
		if (peerBox !== null) {
			peerBox.innerHTML = "<h2>Peers</h2>"
			JSON.parse(nodes[0]).sort((x, y) => x.Address.localeCompare(y.Address)).forEach(n => {
				const elem = document.createElement("div")
				const deleteButton = document.createElement("span")
				deleteButton.appendChild(document.createTextNode("(X) "))
				$(deleteButton).click(function(){
					$.ajax({
						type: 'POST',
						url: "/node",
						data: JSON.stringify(n.Address),
						success: function() {
							update()
						},
						error: function() {
							alert("Unable to delete peer")
						},
						contentType: "application/json"
					})
				})
				let description = ""
				switch (n.Type) {
					case 0:
						description = "manual"
						break
					case 1:
						description = "learned"
						break
					case 2:
						description = "short-circuited"
						break
				}
				elem.appendChild(deleteButton)
				elem.appendChild(document.createTextNode(n.Address + " (" + description + ")"))
				peerBox.appendChild(elem)
			})
		}
		
		const routeBox = document.getElementById("routeContent")
		if (routeBox !== null) {
			routeBox.innerHTML = "<h2>Known nodes</h2>"
			JSON.parse(routes[0]).forEach(route => {
				const elem = document.createElement("div")
				const selectNode = document.createElement("span")
				selectNode.classList.add("button")
				selectNode.appendChild(document.createTextNode(route))
				$(selectNode).click(function() {
					if (!$('*[data-nodename="'+ route +'"]').exists()) {
						$("#tabs ul").append('<li data-nodename="' + route + '"><a href="#tabs-' + tabCounter + '">' + route + '</a> <span>x&nbsp;</span></li></ul>')
						$("#tabs").append('<div data-nodename="' + route + '" id="tabs-'+tabCounter+'"></div>')
						$("#tabs").tabs("refresh")
						$("#tabs ul li span").click(function() {
							const name = $(this).parent("li").attr('data-nodename')
							$('*[data-nodename="'+ name +'"]').remove()
						})
						tabCounter++
					}
				})
				elem.appendChild(selectNode)
				routeBox.appendChild(elem)
				
				$('div[data-nodename="'+ route +'"]').each(function() {
					const that = $(this)
					$.ajax({
						type: 'GET',
						url: "/privateMessage",
						data: {'name': route},
						success: function(result) {
							showMessages(that.get(0), JSON.parse(result), name)
						},
						error: function() {
							alert("Unable to get private messages")
						},
						contentType: "application/json"
					})
				})
				
			})
		}
		
	}, function() {
		//alert("Unable to connect")
	})
}

