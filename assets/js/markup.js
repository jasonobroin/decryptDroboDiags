// Handle hiding our index and navigation link tags based on overall page configuration
// Need a capability to hide/show all diag sections

$(document).ready(function(){
	// Handle a button press on the toggle index button to hide/show all indexing
	$("#toggle-btn").click(function(event){
		
	  $('.linkedindex').toggle()
	  $('.linkedindexdisp').toggle()

	  // When toggling on/off whole indexing, we should display all sections	
		
      // All diag sections marked as open
      $('.open-btn').toggleClass('glyphicon-minus-sign', true);
      $('.open-btn').toggleClass('glyphicon-plus-sign', false);

      // Show all diag sections
	  $('.diag-section').show()
	
    return false;
	});

	// Handle a button press on the hide all button to hide all diag sections
	$("#hide-btn").click(function(event){
			
      // All diag sections marked as closed
      $('.open-btn').toggleClass('glyphicon-minus-sign', false);
      $('.open-btn').toggleClass('glyphicon-plus-sign', true);

      // Hide all diag sections
	  $('.diag-section').hide()
	
    return false;
	});

	// Handle a button press on the show all button to show all diag sections
	$("#show-btn").click(function(event){
			
      // All diag sections marked as open
      $('.open-btn').toggleClass('glyphicon-minus-sign', true);
      $('.open-btn').toggleClass('glyphicon-plus-sign', false);

      // Show all diag sections
	  $('.diag-section').show()
	
    return false;
	});

	// Handle a button press on the reset default style button
	$("#reset-style").click(function(event){
		
	  // Call common code with switch_style...
      $('#userstyle').attr("href", "/assets/styles/default.css");			
	
    return false;
	});

    // Handle the style selection dropdown selection
	$(".switch-style").click(function(event){
	  var style = $(this).text(); 
	
      $('#userstyle').attr("href", "/assets/styles/" + style + ".css");
      // Close the dropdown box
      event.preventDefault();

	  // Store a cookie for the requested style?
	
  	return true;
    });

	// Handle a button press on a toggle button to show/hide diag sections
	$(".display-toggle").click(function(event){		
	  var linkref = $(this).data('section'); // Extract info from data-* attributes

      $(this).find('span').toggleClass('glyphicon-minus-sign glyphicon-plus-sign');
	  $(linkref).toggle()

	return false;
	});

});