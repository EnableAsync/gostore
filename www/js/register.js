function validate() {
    var pwd1 = $("#pwd").val();
    var pwd2 = $("#pwd2").val();
    $('#pwdnote').html(pwd1 + ' ' + pwd2);
    if(pwd1 !== pwd2) {
        // $('#pwddiv').addClass('invalid-feedback');  //  valid-tooltip valid-feedback is-valid
        $('#pwd2').addClass('is-invalid');
        $('#register').attr('disabled', 'disabled');
        $('#pwddiv').removeClass('was-validated');
    } else {
        $('#pwd2').removeClass('is-invalid');
        $('#register').removeAttr('disabled');
        $('#pwddiv').addClass('was-validated');
    }
}

function checkuser() {
    var user = $('#name').val();
    var geturl = 'http://127.0.0.1:8081/register/checkUser';
    $.ajax({
        url: geturl,
        data: {name: user},
        dataType: 'json',
        cache: false,
        success: function (result) {
            if (result.code === 0) {
                $('#name').removeClass('is-invalid');
                $('#register').removeAttr('disabled');
                $('#name').removeClass('title');
                $('#regdiv').addClass('was-validated');
            } else {
                $('#name').addClass('is-invalid');
                $('#register').attr('disabled', 'disabled');
                $('#regdiv').removeClass('was-validated');
                $('#name').attr('title', '用户名已被使用');
            }
        }
    })

}
