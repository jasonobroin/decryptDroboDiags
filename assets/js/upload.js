// Handle a modal dialog when selecting the JIRA upload option; we extract the linkref attribute (as its specific to each table entry)
// and update the modal button action to send a request to the correct URL

$(document).ready(function(){
    // Handle opening a modal dialog
	$('#jirasave').on('show.bs.modal', function (event) {
	  var link = $(event.relatedTarget); // Button that triggered the modal
	  var linkref = link.data('linkref'); // Extract info from data-* attributes
	  var name = link.data('name'); // Extract info from data-* attributes
	  // Update the modal's content. We'll use jQuery here
	  var modal = $(this);
	  modal.find('.modal-title').text('Upload ' + name + ' to JIRA');
	  var jirainfo = $("form.jira-info");
	  $(jirainfo).find("#filename").val(linkref);
	});

    // Handle closing a modal dialog and resetting it to default
	$('#jirasave').on('hidden.bs.modal', function (event) {
      $('.modal-body').find("textarea, input").val('');
	});

	// Handle a button press on the JIRA upload modal to post data to the server
	$("#jirapost-btn").click(function(event){
		
	  // Stop form from submitting normally
      event.preventDefault();

      // Indicate that upload is in progress
	  $('#jirauploadalert').show()
		
	  var jirainfo = $("form.jira-info");
	  $.post("/jira/",$(jirainfo).serialize(),
	  function(data, status){
	    $('#jirauploadalert').hide()	
	    // Dismiss modal dialog
	    $('#jirasave').modal('hide')
	  });	
	return false;
	});
	
	// Hide alert when webpage loaded
	$('#jirauploadalert').hide()
		
});